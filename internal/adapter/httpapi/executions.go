// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// ExecutionReader is what the read side of the executions needs from the store: the reads of an
// execution inside one organization. Both storage adapters implement it.
type ExecutionReader interface {
	GetExecution(ctx context.Context, org orgscope.Org, execution store.ExecutionID) (store.Execution, error)
	ListExecutions(ctx context.Context, org orgscope.Org, filter store.ExecutionFilter, after store.Cursor, limit int) ([]store.Execution, error)
	ListNodeExecutions(ctx context.Context, org orgscope.Org, execution store.ExecutionID, after store.Cursor, limit int) ([]store.NodeExecution, error)
	ListNodeOutputs(ctx context.Context, org orgscope.Org, execution store.ExecutionID, after store.Cursor, limit int) ([]store.NodeOutputRecord, error)
	ListExecutionEvents(ctx context.Context, org orgscope.Org, execution store.ExecutionID, after int64, limit int) ([]store.Event, error)
}

// ExecutionRevisions reads the flow revision an execution pinned: the published version or the
// snapshot of a test run.
type ExecutionRevisions interface {
	GetVersion(ctx context.Context, org orgscope.Org, version flow.FlowVersionID) (flow.VersionRecord, error)
	GetSnapshot(ctx context.Context, org orgscope.Org, snapshot flow.FlowSnapshotID) (flow.SnapshotRecord, error)
}

// FlowDocumentReader turns the stored bytes of a revision into a flow document.
type FlowDocumentReader interface {
	Read(data []byte) (*flow.Document, error)
}

// ConnectorPackages reads the installed connector packages. The data reads use it to learn which
// fields the pinned connector version declares as sensitive.
type ConnectorPackages interface {
	GetPackage(ctx context.Context, connectorID, version string) (connector.PackageRecord, error)
}

// OrgAuditLog stores an entry of an organization chain in its own transaction. The reads of
// execution data use it, because they change nothing else.
type OrgAuditLog interface {
	AppendOrgEntry(ctx context.Context, org orgscope.Org, entry audit.Entry) error
}

// Executions serves the read side of the executions (SPEC-005 section 3.6): listExecutions,
// getExecution, listExecutionNodes, listExecutionData and listExecutionEvents. Each one runs
// after authorization, in the organization the authorization decided, and an execution of
// another organization is as absent as one that does not exist.
type Executions struct {
	reader     ExecutionReader
	revisions  ExecutionRevisions
	docs       FlowDocumentReader
	packages   ConnectorPackages
	authorizer authz.Authorizer
	audit      OrgAuditLog
	clock      clock.Clock
	ids        id.Generator
	codec      *page.Codec
}

// NewExecutions returns the handlers over the store, the flow revisions, the authorizer that
// decides who reads execution data, the audit log, the clock and the page-token codec.
func NewExecutions(reader ExecutionReader, revisions ExecutionRevisions, docs FlowDocumentReader, packages ConnectorPackages, authorizer authz.Authorizer, auditLog OrgAuditLog, clk clock.Clock, ids id.Generator, codec *page.Codec) *Executions {
	return &Executions{reader: reader, revisions: revisions, docs: docs, packages: packages, authorizer: authorizer, audit: auditLog, clock: clk, ids: ids, codec: codec}
}

// maxDataItems is the most items one data entry shows (the contract bounds dataItems).
const maxDataItems = 200

var executionSortFields = []string{"createdAt"}

// readDataPermission is the permission that shows the data of an execution.
const readDataPermission authz.Permission = "execution:read-data"

// ListExecutions lists the executions of a project, newest first by default, filtered by flow,
// status and creation time. The list holds metadata only; dataHidden tells whether the caller
// may read the data.
func (x *Executions) ListExecutions(ctx context.Context, req apigen.ListExecutionsRequestObject) (apigen.ListExecutionsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := pageLimit(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	sortRaw := ""
	if req.Params.Sort != nil {
		sortRaw = string(*req.Params.Sort)
	}
	terms, err := page.ParseSort(sortRaw, executionSortFields, "-createdAt")
	if err != nil {
		return nil, err
	}
	project := identity.ProjectID(req.ProjectId)
	var flowID, status, after, before string
	if req.Params.FlowId != nil {
		flowID = req.Params.FlowId.String()
	}
	if req.Params.Status != nil {
		status = string(*req.Params.Status)
	}
	if req.Params.CreatedAfter != nil {
		after = req.Params.CreatedAfter.UTC().Format(time.RFC3339Nano)
	}
	if req.Params.CreatedBefore != nil {
		before = req.Params.CreatedBefore.UTC().Format(time.RFC3339Nano)
	}
	binding := page.Binding("sort="+page.String(terms), "project="+project.String(), "flowId="+flowID,
		"status="+status, "createdAfter="+after, "createdBefore="+before)
	cursor, err := keysetCursor(x.codec, derefString(req.Params.PageToken), org.String(), binding)
	if err != nil {
		return nil, err
	}
	filter := store.ExecutionFilter{ProjectID: project, Oldest: !terms[0].Descending}
	if req.Params.FlowId != nil {
		filter.FlowID = flow.FlowID(*req.Params.FlowId)
	}
	if req.Params.Status != nil {
		filter.Status = string(*req.Params.Status)
	}
	if req.Params.CreatedAfter != nil {
		filter.CreatedAfter = *req.Params.CreatedAfter
	}
	if req.Params.CreatedBefore != nil {
		filter.CreatedBefore = *req.Params.CreatedBefore
	}
	rows, err := x.reader.ListExecutions(ctx, org, filter, cursor, limit+1)
	if err != nil {
		return nil, wrapReadError(err)
	}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}

	hidden, err := x.dataHidden(ctx, authz.ResourceRef{Kind: "project", ID: project.String()})
	if err != nil {
		return nil, err
	}
	versions := newVersionNumbers(x.revisions, org)
	body := apigen.ExecutionList{Items: []apigen.Execution{}}
	for i := range rows {
		b, err := executionBody(ctx, &rows[i], hidden, versions)
		if err != nil {
			return nil, err
		}
		body.Items = append(body.Items, b)
	}
	if more {
		last := rows[len(rows)-1]
		token, err := tokenFor(x.codec, org.String(), binding, keysetPosition(last.CreatedAt, id.UUID(last.ID).String()))
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &token
	}
	return apigen.ListExecutions200JSONResponse{Body: body}, nil
}

// GetExecution returns one execution with its status. The output summary is not served yet, so
// dataHidden only says whether the caller could read data.
func (x *Executions) GetExecution(ctx context.Context, req apigen.GetExecutionRequestObject) (apigen.GetExecutionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	rec, err := x.load(ctx, org, req.ExecutionId)
	if err != nil {
		return nil, err
	}
	hidden, err := x.dataHidden(ctx, authz.ResourceRef{Kind: "execution", ID: rec.ID.String()})
	if err != nil {
		return nil, err
	}
	body, err := executionBody(ctx, &rec, hidden, newVersionNumbers(x.revisions, org))
	if err != nil {
		return nil, err
	}
	return apigen.GetExecution200JSONResponse{Body: body}, nil
}

// ListExecutionNodes lists the node executions of an execution, oldest first: what each node
// did and how many attempts it took, never its data.
func (x *Executions) ListExecutionNodes(ctx context.Context, req apigen.ListExecutionNodesRequestObject) (apigen.ListExecutionNodesResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := pageLimit(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	rec, err := x.load(ctx, org, req.ExecutionId)
	if err != nil {
		return nil, err
	}
	binding := page.Binding("nodes", "execution="+rec.ID.String())
	cursor, err := keysetCursor(x.codec, derefString(req.Params.PageToken), org.String(), binding)
	if err != nil {
		return nil, err
	}
	rows, err := x.reader.ListNodeExecutions(ctx, org, rec.ID, cursor, limit+1)
	if err != nil {
		return nil, wrapReadError(err)
	}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	body := apigen.NodeExecutionList{Items: make([]apigen.NodeExecution, 0, len(rows))}
	for i := range rows {
		n := &rows[i]
		nid := types.UUID(n.ID)
		item := apigen.NodeExecution{
			Id: &nid, ExecutionId: types.UUID(n.ExecutionID), NodeId: n.NodeID,
			Status: apigen.NodeExecutionStatus(n.Status), AttemptCount: int(n.AttemptCount),
			StartedAt: optionalTime(n.StartedAt), FinishedAt: optionalTime(n.FinishedAt),
		}
		body.Items = append(body.Items, item)
	}
	if more {
		last := rows[len(rows)-1]
		token, err := tokenFor(x.codec, org.String(), binding, keysetPosition(last.CreatedAt, last.ID.String()))
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &token
	}
	return apigen.ListExecutionNodes200JSONResponse{Body: body}, nil
}

// ListExecutionEvents lists the history of an execution in order. An event names the node and
// the edge it is about and never carries execution data.
func (x *Executions) ListExecutionEvents(ctx context.Context, req apigen.ListExecutionEventsRequestObject) (apigen.ListExecutionEventsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := pageLimit(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	rec, err := x.load(ctx, org, req.ExecutionId)
	if err != nil {
		return nil, err
	}
	binding := page.Binding("events", "execution="+rec.ID.String())
	var afterSeq int64
	if token := derefString(req.Params.PageToken); token != "" {
		cur, err := x.codec.Decode(token, org.String(), binding)
		if err != nil {
			return nil, err
		}
		if len(cur.Position) != 1 {
			return nil, page.ErrInvalidToken
		}
		afterSeq, err = strconv.ParseInt(cur.Position[0], 10, 64)
		if err != nil || afterSeq < 0 {
			return nil, page.ErrInvalidToken
		}
	}
	rows, err := x.reader.ListExecutionEvents(ctx, org, rec.ID, afterSeq, limit+1)
	if err != nil {
		return nil, wrapReadError(err)
	}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	body := apigen.ExecutionEventList{Items: make([]apigen.ExecutionEvent, 0, len(rows))}
	for i := range rows {
		ev := &rows[i]
		item := apigen.ExecutionEvent{SequenceNumber: int(ev.SequenceNumber), Type: ev.Type, OccurredAt: ev.OccurredAt}
		node, edge := eventSubject(ev.Reference)
		if node != "" {
			item.NodeId = &node
		}
		if edge != "" {
			item.EdgeId = &edge
		}
		body.Items = append(body.Items, item)
	}
	if more {
		token, err := x.codec.Encode(org.String(), binding, page.Cursor{Position: []string{strconv.FormatInt(rows[len(rows)-1].SequenceNumber, 10)}})
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &token
	}
	return apigen.ListExecutionEvents200JSONResponse{Body: body}, nil
}

// ListExecutionData lists the items each node produced, one entry per node execution and port.
// It needs execution:read-data, which the pipeline checked; the values the flow marks as
// sensitive leave as the mask (ADR-0017 section 3), and the first page of every read is audited
// as execution.data_viewed (ADR-0049 section 3). Nothing is served unmasked: when the marks
// cannot be read, the request fails.
func (x *Executions) ListExecutionData(ctx context.Context, req apigen.ListExecutionDataRequestObject) (apigen.ListExecutionDataResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok {
		return nil, identity.ErrUnauthenticated
	}
	limit, err := pageLimit(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	rec, err := x.load(ctx, org, req.ExecutionId)
	if err != nil {
		return nil, err
	}
	binding := page.Binding("data", "execution="+rec.ID.String())
	token := derefString(req.Params.PageToken)
	cursor, err := keysetCursor(x.codec, token, org.String(), binding)
	if err != nil {
		return nil, err
	}
	marks, err := x.sensitivePaths(ctx, org, &rec)
	if err != nil {
		return nil, err
	}
	rows, err := x.reader.ListNodeOutputs(ctx, org, rec.ID, cursor, limit+1)
	if err != nil {
		return nil, wrapReadError(err)
	}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	body := apigen.ExecutionDataList{Items: make([]apigen.ExecutionDataEntry, 0, len(rows))}
	for i := range rows {
		entry, err := dataEntry(&rows[i], marks[rows[i].NodeID])
		if err != nil {
			return nil, err
		}
		body.Items = append(body.Items, entry)
	}
	if token == "" {
		// A read of data is audited before the data leaves; when the entry cannot be stored the
		// data is not served.
		if err := x.auditView(ctx, org, principal, rec.ID); err != nil {
			return nil, err
		}
	}
	if more {
		last := rows[len(rows)-1]
		next, err := tokenFor(x.codec, org.String(), binding, keysetPosition(last.CreatedAt, id.UUID(last.ID).String()))
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &next
	}
	return apigen.ListExecutionData200JSONResponse{Body: body}, nil
}

// load reads an execution of the organization; one that is not there is not found.
func (x *Executions) load(ctx context.Context, org orgscope.Org, execID types.UUID) (store.Execution, error) {
	rec, err := x.reader.GetExecution(ctx, org, store.ExecutionID(execID))
	if err != nil {
		return store.Execution{}, wrapReadError(err)
	}
	return rec, nil
}

// dataHidden reports whether the caller lacks the permission to read execution data on the
// resource. The pipeline already authorized the request itself, so a refusal here only hides data.
func (x *Executions) dataHidden(ctx context.Context, resource authz.ResourceRef) (bool, error) {
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok {
		return true, nil
	}
	decision, _, err := x.authorizer.Authorize(ctx, authz.Request{
		Principal:   principal,
		Requirement: authz.Requirement{Mode: authz.ModeAll, Permissions: []authz.Permission{readDataPermission}},
		Resource:    resource,
	})
	if err != nil {
		return true, err
	}
	return decision != authz.DecisionAllow, nil
}

// sensitivePaths returns, by node id, the paths that are sensitive for the execution: the paths
// the flow revision marks on the node, together with the fields the manifest of the connector
// version the node pins declares as sensitive (ADR-0017 section 3). When a manifest cannot be read
// the call fails, because the data could not be masked.
func (x *Executions) sensitivePaths(ctx context.Context, org orgscope.Org, rec *store.Execution) (map[string][]string, error) {
	var raw []byte
	switch {
	case !rec.FlowVersionID.IsZero():
		v, err := x.revisions.GetVersion(ctx, org, rec.FlowVersionID)
		if err != nil {
			return nil, fmt.Errorf("read the flow version of the execution: %w", err)
		}
		raw = v.Document
	case !rec.FlowSnapshotID.IsZero():
		s, err := x.revisions.GetSnapshot(ctx, org, rec.FlowSnapshotID)
		if err != nil {
			return nil, fmt.Errorf("read the flow snapshot of the execution: %w", err)
		}
		raw = s.Document
	default:
		return nil, errors.New("the execution pins no flow revision")
	}
	doc, err := x.docs.Read(raw)
	if err != nil {
		return nil, err
	}
	marks := make(map[string][]string, len(doc.Nodes))
	manifests := map[flow.ConnectorUse]connector.Manifest{}
	for i := range doc.Nodes {
		n := &doc.Nodes[i]
		paths := slices.Clone(n.SensitivePaths)
		if n.Connector != nil {
			declared, err := x.connectorSensitiveFields(ctx, manifests, n)
			if err != nil {
				return nil, err
			}
			paths = append(paths, declared...)
		}
		if len(paths) > 0 {
			marks[string(n.ID)] = paths
		}
	}
	return marks, nil
}

// connectorSensitiveFields returns the sensitive fields the pinned connector version declares for
// the type of the node. A package that is missing, is not the pinned one, cannot be read or does
// not declare the node type is an error: the output of the node could not be masked.
func (x *Executions) connectorSensitiveFields(ctx context.Context, cache map[flow.ConnectorUse]connector.Manifest, n *flow.Node) ([]string, error) {
	connectorID, ok := flow.ConnectorIDOf(n.Type)
	if !ok {
		return nil, fmt.Errorf("the node %q pins a connector but its type names none", n.ID)
	}
	use := flow.ConnectorUse{ConnectorID: connectorID, ConnectorVersion: n.Connector.Version}
	m, ok := cache[use]
	if !ok {
		rec, err := x.packages.GetPackage(ctx, use.ConnectorID, use.ConnectorVersion)
		if errors.Is(err, connector.ErrNotFound) {
			// The execution exists; a missing package is a fault of the installation, not a 404.
			return nil, fmt.Errorf("the connector %s %s the execution pins is not installed", use.ConnectorID, use.ConnectorVersion)
		}
		if err != nil {
			return nil, fmt.Errorf("read the connector %s %s the execution pins: %w", use.ConnectorID, use.ConnectorVersion, err)
		}
		if n.Connector.Digest != "" && rec.Digest != n.Connector.Digest {
			return nil, fmt.Errorf("the connector %s %s installed is not the package the flow pins", use.ConnectorID, use.ConnectorVersion)
		}
		m, err = connector.Parse(rec.Manifest, connector.Options{})
		if err != nil {
			return nil, fmt.Errorf("read the manifest of the connector %s %s: %w", use.ConnectorID, use.ConnectorVersion, err)
		}
		cache[use] = m
	}
	for i := range m.Nodes {
		if m.Nodes[i].Type == n.Type {
			return m.Nodes[i].SensitiveFields, nil
		}
	}
	return nil, fmt.Errorf("the connector %s %s declares no node of type %s", use.ConnectorID, use.ConnectorVersion, n.Type)
}

// auditView appends execution.data_viewed. The details name the execution and no data.
func (x *Executions) auditView(ctx context.Context, org orgscope.Org, principal identity.Principal, exec store.ExecutionID) error {
	details, err := jsonvalue.Object(jsonvalue.Member{Key: "executionId", Value: jsonvalue.String(exec.String())})
	if err != nil {
		return fmt.Errorf("audit the data read: %w", err)
	}
	entry, err := audit.NewEntry(audit.EntryParams{
		ID: x.ids.New(), Chain: audit.OrganizationChain(org.UUID()), OccurredAt: clock.Micros(x.clock.Now()),
		Actor: actorOf(principal), Action: "execution.data_viewed", Outcome: audit.OutcomeSuccess,
		RequestID: middleware.RequestIDFrom(ctx), Details: details,
	})
	if err != nil {
		return fmt.Errorf("audit the data read: %w", err)
	}
	if err := x.audit.AppendOrgEntry(ctx, org, entry); err != nil {
		return fmt.Errorf("audit the data read: %w", err)
	}
	return nil
}

// dataEntry is one output batch as the API shows it: at most maxDataItems items with the
// sensitive paths of its node masked.
func dataEntry(rec *store.NodeOutputRecord, paths []string) (apigen.ExecutionDataEntry, error) {
	entry := apigen.ExecutionDataEntry{
		NodeExecutionId: types.UUID(rec.NodeExecutionID), NodeId: rec.NodeID, Side: apigen.OUTPUT,
		Port: rec.Port, ItemCount: int(rec.ItemCount), DataItems: []map[string]any{},
	}
	if len(rec.Document) == 0 {
		return entry, nil
	}
	batch, err := execution.DecodeBatch(rec.Document)
	if err != nil {
		return apigen.ExecutionDataEntry{}, err
	}
	for i, it := range batch.Items() {
		if i == maxDataItems {
			break
		}
		obj, err := itemObject(execution.MaskPaths(it.JSON, paths))
		if err != nil {
			return apigen.ExecutionDataEntry{}, err
		}
		entry.DataItems = append(entry.DataItems, obj)
	}
	return entry, nil
}

// itemObject converts the json of an item to the object the response carries. Numbers keep their
// text. A value that is not an object is shown under the member "value".
func itemObject(v jsonvalue.Value) (map[string]any, error) {
	raw, err := jsonvalue.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode an item: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var decoded any
	if err := dec.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode an item: %w", err)
	}
	if obj, ok := decoded.(map[string]any); ok {
		return obj, nil
	}
	return map[string]any{"value": decoded}, nil
}

// eventSubject reads the node and the edge an event is about from its reference document. A
// reference that cannot be read names neither.
func eventSubject(reference []byte) (node, edge string) {
	var ref struct {
		NodeID string `json:"nodeId"`
		EdgeID string `json:"edgeId"`
	}
	if err := json.Unmarshal(reference, &ref); err != nil {
		return "", ""
	}
	return ref.NodeID, ref.EdgeID
}

// versionNumbers resolves the number of the flow version an execution runs, once per version
// for one request.
type versionNumbers struct {
	revisions ExecutionRevisions
	org       orgscope.Org
	seen      map[flow.FlowVersionID]int
}

func newVersionNumbers(revisions ExecutionRevisions, org orgscope.Org) *versionNumbers {
	return &versionNumbers{revisions: revisions, org: org, seen: map[flow.FlowVersionID]int{}}
}

func (v *versionNumbers) of(ctx context.Context, rec *store.Execution) (int, error) {
	if rec.FlowVersionID.IsZero() {
		// A test run pins a snapshot of the draft, which has no version number; 1 is the least
		// the contract allows.
		return 1, nil
	}
	if n, ok := v.seen[rec.FlowVersionID]; ok {
		return n, nil
	}
	rv, err := v.revisions.GetVersion(ctx, v.org, rec.FlowVersionID)
	if err != nil {
		return 0, fmt.Errorf("read the flow version of an execution: %w", err)
	}
	n := int(rv.Number)
	v.seen[rec.FlowVersionID] = n
	return n, nil
}

// executionBody is the execution as the API shows it.
func executionBody(ctx context.Context, rec *store.Execution, hidden bool, versions *versionNumbers) (apigen.Execution, error) {
	number, err := versions.of(ctx, rec)
	if err != nil {
		return apigen.Execution{}, err
	}
	eid := types.UUID(rec.ID)
	return apigen.Execution{
		Id: &eid, FlowId: types.UUID(rec.FlowID), ProjectId: types.UUID(rec.ProjectID), FlowVersion: number,
		Mode: apigen.ExecutionMode(rec.Mode), Status: apigen.ExecutionStatus(rec.Status), CreatedAt: rec.CreatedAt,
		StartedAt: optionalTime(rec.StartedAt), FinishedAt: optionalTime(rec.FinishedAt), DataHidden: hidden,
	}, nil
}

// optionalTime is nil for the zero time.
func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// keysetPosition is the position after a row of a list in creation order.
func keysetPosition(createdAt time.Time, rowID string) []string {
	return []string{sortKeyTime(createdAt), rowID}
}

// keysetCursor decodes the page token of a list in creation order; no token starts at the first
// row. A token of another organization or list is page.ErrInvalidToken.
func keysetCursor(codec *page.Codec, token, org, binding string) (store.Cursor, error) {
	if token == "" {
		return store.Cursor{}, nil
	}
	cur, err := codec.Decode(token, org, binding)
	if err != nil {
		return store.Cursor{}, err
	}
	if len(cur.Position) != 2 {
		return store.Cursor{}, page.ErrInvalidToken
	}
	micros, err := strconv.ParseInt(cur.Position[0], 10, 64)
	if err != nil {
		return store.Cursor{}, page.ErrInvalidToken
	}
	rowID, err := id.Parse(cur.Position[1])
	if err != nil {
		return store.Cursor{}, page.ErrInvalidToken
	}
	return store.Cursor{CreatedAt: time.UnixMicro(micros).UTC(), ID: rowID}, nil
}

// wrapReadError maps the not-found of the execution store to the one the API answers with 404.
func wrapReadError(err error) error {
	if errors.Is(err, flow.ErrNotFound) {
		return identity.ErrNotFound
	}
	return err
}
