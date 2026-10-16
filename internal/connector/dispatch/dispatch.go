// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package dispatch runs the nodes that act on external systems (SPEC-003
// CN3-dispatch): the core HTTP node orcon.http@1 (Annex B) and the rules around
// every send. A send is either a read, which is repeated freely, or an effect,
// which goes through the external operation journal (ADR-0012): intent first,
// the DISPATCHING mark committed before the request, the outcome recorded after,
// and an unknown outcome left UNCERTAIN instead of being repeated. Every send
// first takes permits from the shared rate bucket of its credential and host
// (ADR-0014 section 2.4); the credential is applied by the declarative
// interpreter, which keeps it out of items, logs and results.
//
// The package is an area package: the network is the [Config.SenderFor] port,
// the journal and bucket stores are ports, and nothing here imports an adapter.
package dispatch

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/connector/limit"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/journal"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// HTTPNodeType is the type of the core HTTP node (ADR-0037 Annex B).
const HTTPNodeType = "orcon.http@1"

// IdempotencyHeader carries the key of an idempotent effect (ADR-0079 §4).
const IdempotencyHeader = "Idempotency-Key"

// ErrThrottled means a rate bucket refused the permits. Nothing was sent. The
// node answers it with an [execution.DelayError] that wraps this error, so the
// engine runs the activation again at the instant the bucket names (ADR-0014
// section 2.4) instead of failing it.
var ErrThrottled = errors.New("dispatch: throttled by a rate bucket")

// ErrUnknownOutcome means an effect's outcome is not known and must be settled
// by a person (UNCERTAIN, ADR-0012 section 7). Nothing is sent again. An effect
// answers it with an [execution.SuspendError] that wraps this error, so the
// engine parks the node, the token and the execution until a person decides.
var ErrUnknownOutcome = errors.New("dispatch: outcome of an effect is unknown")

// CodeCredentialHostNotAllowed is the code of the failure raised when the credential a node names is not allowed
// for the host the request goes to (ADR-0043 section 4). The failure is permanent: sending again does not change it.
const CodeCredentialHostNotAllowed = "CredentialHostNotAllowed" //nolint:gosec // an error code, not a credential

// Credential is the credential a node runs under, resolved by the caller: the
// authentication scheme and the decrypted fields. Values never leave the send.
type Credential struct {
	Auth   declarative.Auth
	Fields map[string]string
	// Mask holds every secret value of the credential. The send replaces each
	// occurrence in the response items (strings and object keys) with a redaction
	// marker before the items become node output or journal response (ADR-0043
	// section 5, R-CRED-04). The base64 form of a BASIC pair is derived from Fields.
	Mask []string
}

// CredentialRequest asks for the credential one activation names.
type CredentialRequest struct {
	Org     identity.OrgID
	Project identity.ProjectID
	// ID is the credential the node names.
	ID string
	// Host is the host the request goes to; the resolver refuses a credential
	// that is not allowed for it.
	Host string
}

// Config are the ports and bounds of a dispatcher.
type Config struct {
	Journal journal.Store
	// Scopes issues the scope of the organization of each call, for the journal
	// (ADR-0046 section 2).
	Scopes  store.Scopes
	Buckets store.LimitBuckets
	// SenderFor returns the network port of one destination: the egress
	// adapter, bound to the policy that grants that host.
	SenderFor func(u *url.URL) declarative.Sender
	// Credential resolves the credential a node names, for the project of its
	// execution and the host the request is sent to, on every activation (ADR-0043
	// sections 3 and 4). A nil function means no node can name a credential.
	Credential func(ctx context.Context, req CredentialRequest) (Credential, error)
	// Bucket is the rate bucket of each credential and host.
	Bucket     limit.Spec
	IDs        id.Generator
	Clock      clock.Clock
	JournalCfg journal.Config
	KeyTTL     time.Duration
}

// HTTPNode runs orcon.http@1 for the activations of a flow.
type HTTPNode struct {
	cfg Config
}

// New returns the runner of the core HTTP node. It validates the ports it needs.
func New(cfg Config) (*HTTPNode, error) {
	if cfg.Journal == nil || cfg.Buckets == nil || cfg.SenderFor == nil || cfg.IDs == nil || cfg.Scopes == nil {
		return nil, errors.New("dispatch: journal, buckets, sender, scopes and id ports are required")
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.System{}
	}
	if cfg.KeyTTL <= 0 {
		cfg.KeyTTL = 24 * time.Hour
	}
	if err := cfg.Bucket.Validate(); err != nil {
		return nil, fmt.Errorf("dispatch: bucket: %w", err)
	}
	return &HTTPNode{cfg: cfg}, nil
}

// request is one parsed orcon.http@1 node: what to send, for every item.
type request struct {
	method     string
	target     *url.URL
	headers    map[string]string
	body       string
	credential string
	idempotent bool
	// org and bucket locate the rate bucket the send draws from; they are set
	// by Execute before any send.
	org    identity.OrgID
	bucket string
}

// parse reads the parameters of the node. Header values and the URL may not
// contain template markers: the interpreter would evaluate them.
func parse(node *flow.Node) (request, error) {
	var r request
	p := node.Parameters
	r.method = "GET"
	if v, ok := stringParam(p, "method"); ok && v != "" {
		r.method = strings.ToUpper(v)
	}
	raw, ok := stringParam(p, "url")
	if !ok || raw == "" {
		return r, errors.New("orcon.http@1 needs a url")
	}
	if strings.Contains(raw, "{{") {
		return r, errors.New("orcon.http@1 url may not contain template markers")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return r, errors.New("orcon.http@1 url must be an absolute http or https URL")
	}
	r.target = u
	r.headers = map[string]string{}
	if h, ok := p.Get("headers"); ok {
		for _, m := range h.Members() {
			s, isStr := m.Value.Str()
			if !isStr || strings.Contains(s, "{{") {
				return r, fmt.Errorf("orcon.http@1 header %q must be a string without template markers", m.Key)
			}
			r.headers[m.Key] = s
		}
	}
	if b, ok := p.Get("body"); ok {
		text, err := jsonvalue.Marshal(b)
		if err != nil {
			return r, errors.New("orcon.http@1 body is not encodable")
		}
		r.body = string(text)
	}
	r.credential, _ = stringParam(p, "credential")
	if v, ok := p.Get("idempotent"); ok {
		if b, isBool := v.Bool(); isBool {
			r.idempotent = b
		}
	}
	return r, nil
}

func stringParam(p jsonvalue.Value, name string) (string, bool) {
	v, ok := p.Get(name)
	if !ok {
		return "", false
	}
	return v.Str()
}

// isRead reports whether the method only reads (GET and HEAD).
func (r request) isRead() bool { return r.method == "GET" || r.method == "HEAD" }

// purity and safety of the send (ADR-0029 §5, ADR-0079 §4): reads are
// READ_ONLY; an effect is keyed when the node says so, and UNSAFE otherwise.
func (r request) operation() declarative.Operation {
	op := declarative.Operation{Request: declarative.Request{Method: r.method, Path: pathOf(r.target), Headers: r.headers}}
	if r.isRead() {
		op.Purity, op.Safety = declarative.PurityRead, declarative.SafetyReadOnly
	} else {
		op.Purity = declarative.PurityEffect
		op.Safety = declarative.SafetyUnsafe
		if r.idempotent {
			op.Safety = declarative.SafetyIdempotencyKey
			op.IdempotencyHeader = IdempotencyHeader
			op.IdempotencyKey = "{{$parameter.key}}"
		}
	}
	if r.body != "" {
		op.Request.Body = &declarative.Body{Type: "RAW", Raw: "{{$parameter.body}}"}
		op.Request.Headers = withContentType(op.Request.Headers)
	}
	return op
}

func withContentType(h map[string]string) map[string]string {
	out := map[string]string{"Content-Type": "application/json"}
	for k, v := range h {
		out[k] = v
	}
	return out
}

// pathOf is the path and query of the target, which is what the interpreter
// renders below the base URL.
func pathOf(u *url.URL) string {
	p := u.EscapedPath()
	if p == "" {
		p = "/"
	}
	if u.RawQuery != "" {
		p += "?" + u.RawQuery
	}
	return p
}

// definition is the one-operation connector an item is sent with.
func (r request) definition(auth declarative.Auth) declarative.Definition {
	base := &url.URL{Scheme: r.target.Scheme, Host: r.target.Host}
	return declarative.Definition{
		Format: declarative.Format,
		HTTP:   declarative.HTTP{BaseURL: base.String(), Auth: auth},
		Operations: map[string]declarative.Operation{
			"http.call": r.operation(),
		},
	}
}

// Execute runs orcon.http@1 for one activation: one request per input item. The
// node output is one item per response, on the main port. Failures return an
// error that names the class; nothing of a credential is in it.
func (n *HTTPNode) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	if call.Node.Type != HTTPNodeType {
		return execution.NodeOutput{}, fmt.Errorf("dispatch: node type %q is not %s", call.Node.Type, HTTPNodeType)
	}
	req, err := parse(call.Node)
	if err != nil {
		return execution.NodeOutput{}, err
	}
	cred, err := n.credential(ctx, call, req)
	if err != nil {
		return execution.NodeOutput{}, err
	}
	bucketKey := bucketKeyOf(req.credential, req.target.Host)
	req.org, req.bucket = call.Org, bucketKey
	count := call.Input.Len()
	if count == 0 {
		return execution.NodeOutput{}, nil
	}

	var results []jsonvalue.Value
	if req.isRead() {
		if err := n.takeReadPermits(ctx, call.Org, bucketKey, int64(count)); err != nil {
			return execution.NodeOutput{}, err
		}
		results, err = n.reads(ctx, req, cred, count)
	} else {
		results, err = n.effects(ctx, req, cred, call)
	}
	if err != nil {
		return execution.NodeOutput{}, err
	}
	items := make([]execution.Item, 0, len(results))
	for _, v := range results {
		items = append(items, execution.Item{JSON: v})
	}
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: execution.NewBatch(items...)}}}, nil
}

func (n *HTTPNode) credential(ctx context.Context, call execution.NodeCall, req request) (Credential, error) {
	name := req.credential
	if name == "" {
		return Credential{Auth: declarative.Auth{Type: declarative.AuthNone}}, nil
	}
	if n.cfg.Credential == nil {
		return Credential{}, errors.New("dispatch: no credential resolver is configured")
	}
	return n.cfg.Credential(ctx, CredentialRequest{
		Org: call.Org, Project: call.Project, ID: name, Host: req.target.Hostname(),
	})
}

func bucketKeyOf(credential, host string) string {
	return "http:" + credential + ":" + host
}

// takeChunk takes up to one bucket of permits. When the bucket refuses, nothing
// was taken and the instant of the next permit comes back with granted false.
func (n *HTTPNode) takeChunk(ctx context.Context, org identity.OrgID, key string, permits int64) (granted bool, retryAt time.Time, err error) {
	v, err := n.cfg.Buckets.TakeBucketPermits(ctx, org, store.BucketRequest{
		Key: key, Capacity: n.cfg.Bucket.Capacity, RefillPerSecond: n.cfg.Bucket.RefillPerSecond, Permits: permits, Now: n.cfg.Clock.Now(),
	})
	if err != nil {
		return false, time.Time{}, err
	}
	return v.Granted, v.RetryAt, nil
}

// throttled is the answer of a refused bucket: the activation runs again at the
// instant the bucket offers, and nothing is failed.
func throttled(retryAt time.Time) error {
	return &execution.DelayError{Until: retryAt, Reason: "rate bucket refused the permits", Err: ErrThrottled}
}

// takeReadPermits takes count permits from the bucket, in groups the bucket can
// hold. A refusal of the first group delays the activation: nothing was taken.
// A later group can only be refused when the activation needs more permits than
// the bucket holds, which no repetition of the activation would ever get at
// once, so the activation waits for it in place.
func (n *HTTPNode) takeReadPermits(ctx context.Context, org identity.OrgID, key string, count int64) error {
	for first := true; count > 0; first = false {
		chunk := min(count, n.cfg.Bucket.Capacity)
		granted, retryAt, err := n.takeChunk(ctx, org, key, chunk)
		if err != nil {
			return err
		}
		if granted {
			count -= chunk
			continue
		}
		if first {
			return throttled(retryAt)
		}
		if err := n.waitUntil(ctx, retryAt); err != nil {
			return err
		}
	}
	return nil
}

// waitUntil blocks until the instant or the end of ctx.
func (n *HTTPNode) waitUntil(ctx context.Context, at time.Time) error {
	d := at.Sub(n.cfg.Clock.Now())
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("dispatch: waiting for the rate bucket: %w", ctx.Err())
	case <-t.C:
		return nil
	}
}

// reads sends the request once per input item. A read repeats freely, so it
// needs no journal. The parameters are the same for every item in this version.
func (n *HTTPNode) reads(ctx context.Context, req request, cred Credential, count int) ([]jsonvalue.Value, error) {
	out := make([]jsonvalue.Value, 0, count)
	for range count {
		v, err := n.sendOnce(ctx, req, cred, "")
		if err != nil {
			return nil, err
		}
		out = append(out, v...)
	}
	return out, nil
}

// sendOnce runs the one-operation connector and returns its items.
func (n *HTTPNode) sendOnce(ctx context.Context, req request, cred Credential, key string) ([]jsonvalue.Value, error) {
	def := req.definition(cred.Auth)
	in, err := declarative.New(def, declarative.Options{Sender: n.cfg.SenderFor(req.target), Now: n.cfg.Clock.Now})
	if err != nil {
		return nil, fmt.Errorf("dispatch: %w", err)
	}
	res, err := in.Execute(ctx, n.call(req, cred, key), nil)
	if err != nil {
		n.honorRetryAfter(ctx, req, err)
		return nil, describe(err)
	}
	return newMasker(cred).items(res.Items), nil
}

// honorRetryAfter blocks the shared bucket until the instant the provider asked
// for (a 429 or a Retry-After header). It is best effort: a failed block only
// means the next send is tried, and the provider refuses it again if needed.
func (n *HTTPNode) honorRetryAfter(ctx context.Context, req request, err error) {
	var f *declarative.CallError
	if !errors.As(err, &f) || f.RetryAfter <= 0 {
		return
	}
	now := n.cfg.Clock.Now()
	_ = n.cfg.Buckets.BlockBucket(ctx, req.org, req.bucket, now.Add(f.RetryAfter), now)
}

func (n *HTTPNode) call(req request, cred Credential, key string) declarative.Call {
	params := map[string]jsonvalue.Value{}
	if key != "" {
		params["key"] = jsonvalue.String(key)
	}
	if req.body != "" {
		params["body"] = jsonvalue.String(req.body)
	}
	return declarative.Call{Operation: "http.call", Parameters: params, Credentials: cred.Fields}
}

// describe turns an interpreter failure into an error without any value of the
// request in it.
func describe(err error) error {
	var f *declarative.CallError
	if errors.As(err, &f) {
		if f.Ambiguous {
			return fmt.Errorf("%w: %s %s", ErrUnknownOutcome, f.Class, f.Code)
		}
		return fmt.Errorf("dispatch: %s %s", f.Class, f.Code)
	}
	return fmt.Errorf("dispatch: %w", err)
}

// effects runs an effect through the journal. The intent is written inside the
// checkpoint of the activation, before any send (ADR-0012 section 4). What the
// journal holds then decides each item, so a re-run of the activation after a
// crash never sends an operation twice (ADR-0012 sections 6 and 7):
//   - a PREPARED item is dispatched;
//   - a DISPATCHING item goes to recovery: it is resent with its key while the key
//     lives, otherwise it becomes UNCERTAIN;
//   - a SUCCEEDED item reuses the response the journal stored, and nothing is sent.
func (n *HTTPNode) effects(ctx context.Context, req request, cred Credential, call execution.NodeCall) ([]jsonvalue.Value, error) {
	count := call.Input.Len()
	now := n.cfg.Clock.Now()
	execID, _, nodeExecID, err := storeIdentities(call)
	if err != nil {
		return nil, err
	}
	scope, err := n.cfg.Scopes.ScopeOf(call.Org)
	if err != nil {
		return nil, err
	}
	keys := make([]string, count)
	ops := make([]store.ExternalOperation, count)
	for i := range count {
		keys[i] = journal.CorrelationKey(string(call.Execution), string(call.Node.ID), string(call.Scope), string(call.Token), int64(i), 0)
		ops[i] = store.ExternalOperation{
			ID:                store.ExternalOperationID(n.cfg.IDs.New()),
			OrgID:             call.Org,
			ExecutionID:       execID,
			NodeExecutionID:   nodeExecID,
			ItemIndex:         int64(i),
			CorrelationKey:    keys[i],
			ContractReference: HTTPNodeType,
			CreatedAt:         now,
		}
	}
	contract := journal.Contract{Idempotency: journal.IdempotencyNone}
	if req.idempotent {
		contract = journal.Contract{Idempotency: journal.IdempotencyProviderKey, KeyTTL: n.cfg.KeyTTL}
	}
	p := &effectProvider{n: n, req: req, cred: cred, results: make([]*jsonvalue.Value, count)}
	fenced := journalStore{base: n.cfg.Journal, checkpoint: call.Checkpoint}
	j, err := journal.New(fenced, p, n.cfg.Clock, n.cfg.JournalCfg)
	if err != nil {
		return nil, err
	}
	if err := fenced.InTx(ctx, scope, func(ctx context.Context, tx store.Tx) error {
		return j.PrepareTx(ctx, tx, ops)
	}); err != nil {
		return nil, err
	}
	// The rows the journal holds are the truth: after a crash they may carry the ids of an earlier attempt.
	rows, err := operationsOf(ctx, fenced, scope, nodeExecID, keys)
	if err != nil {
		return nil, err
	}
	var prepared []store.ExternalOperation
	recovering := false
	for i := range rows {
		switch rows[i].State {
		case journal.StatePrepared:
			prepared = append(prepared, rows[i])
		case journal.StateDispatching:
			recovering = true
		}
	}
	// Each group of the dispatch takes its permits first. A refusal delays the
	// activation: the groups already sent are SUCCEEDED in the journal, and the
	// run that follows sends only what is still PREPARED.
	for len(prepared) > 0 {
		chunk := prepared[:min(int64(len(prepared)), n.cfg.Bucket.Capacity)]
		prepared = prepared[len(chunk):]
		granted, retryAt, err := n.takeChunk(ctx, call.Org, req.bucket, int64(len(chunk)))
		if err != nil {
			return nil, err
		}
		if !granted {
			return nil, throttled(retryAt)
		}
		if _, err := j.Dispatch(ctx, scope, chunk, contract); err != nil {
			return nil, err
		}
	}
	if recovering {
		if _, err := j.Recover(ctx, scope, nodeExecID, contract); err != nil {
			return nil, err
		}
	}
	final, err := operationsOf(ctx, fenced, scope, nodeExecID, keys)
	if err != nil {
		return nil, err
	}
	if at, waiting := n.recoveryInstant(final); waiting {
		return nil, &execution.DelayError{Until: at, Reason: "an effect awaits its recovery window", Err: errRecoveryPending}
	}
	return outputsOf(final, p.results)
}

// errRecoveryPending is the cause of the delay of an activation that found items
// still DISPATCHING: the worker that sent them may be alive until the dispatch
// deadline plus the grace has passed (ADR-0012 section 6).
var errRecoveryPending = errors.New("dispatch: effect items are still DISPATCHING")

// recoveryInstant reports whether items are left DISPATCHING after the recovery
// and the instant to try it again: the end of the earliest recovery window, or
// one grace from now for an item whose window is over but whose resend did not
// settle (a transport error keeps it DISPATCHING for the next recovery).
func (n *HTTPNode) recoveryInstant(rows []store.ExternalOperation) (time.Time, bool) {
	grace := n.cfg.JournalCfg.Grace
	if grace <= 0 {
		grace = defaultGrace
	}
	now := n.cfg.Clock.Now()
	var at time.Time
	waiting := false
	for i := range rows {
		if rows[i].State != journal.StateDispatching {
			continue
		}
		due := rows[i].DispatchDeadlineAt.Add(grace)
		if !due.After(now) {
			due = now.Add(grace)
		}
		if !waiting || due.Before(at) {
			at = due
		}
		waiting = true
	}
	return at, waiting
}

// defaultGrace is the grace the journal assumes when its configuration gives none.
const defaultGrace = 5 * time.Second

// operationsOf returns the rows of a node execution for the given correlation keys,
// in key order. A missing row is an error: the checkpoint writes every intent.
func operationsOf(ctx context.Context, s journalStore, org orgscope.Org, node store.NodeExecutionID, keys []string) ([]store.ExternalOperation, error) {
	byKey := make(map[string]store.ExternalOperation, len(keys))
	var fromItem int64
	var fromID store.ExternalOperationID
	for {
		page, err := s.base.ListExternalOperations(ctx, org, node, fromItem, fromID, operationPage)
		if err != nil {
			return nil, fmt.Errorf("dispatch: list operations: %w", err)
		}
		for i := range page {
			byKey[page[i].CorrelationKey] = page[i]
		}
		if len(page) < operationPage {
			break
		}
		last := page[len(page)-1]
		fromItem, fromID = last.ItemIndex, last.ID
	}
	out := make([]store.ExternalOperation, len(keys))
	for i, k := range keys {
		row, ok := byKey[k]
		if !ok {
			return nil, fmt.Errorf("dispatch: item %d has no journal row", i)
		}
		out[i] = row
	}
	return out, nil
}

// outputsOf turns the settled items into the node's output, in item order. A send
// made by this run is in results; an item settled by an earlier run is read from
// the response the journal stored.
func outputsOf(rows []store.ExternalOperation, results []*jsonvalue.Value) ([]jsonvalue.Value, error) {
	out := make([]jsonvalue.Value, 0, len(rows))
	uncertain := 0
	for i := range rows {
		row := rows[i]
		switch row.State {
		case journal.StateSucceeded:
			if r := results[row.ItemIndex]; r != nil {
				out = append(out, *r)
				continue
			}
			v, err := storedResponse(row.ResponseReference)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		case journal.StateUncertain:
			uncertain++
		case journal.StateFailed:
			return nil, fmt.Errorf("dispatch: item %d failed", row.ItemIndex)
		default:
			return nil, fmt.Errorf("dispatch: item %d is %s after the dispatch", row.ItemIndex, row.State)
		}
	}
	if uncertain > 0 {
		return nil, &execution.SuspendError{
			Reason: fmt.Sprintf("%d item(s) of the effect have an unknown outcome", uncertain),
			Err:    fmt.Errorf("%w: %d item(s)", ErrUnknownOutcome, uncertain),
		}
	}
	return out, nil
}

// storedResponse reads the response the journal kept for a succeeded item.
func storedResponse(text string) (jsonvalue.Value, error) {
	if text == "" {
		return jsonvalue.Null(), nil
	}
	v, err := jsonvalue.Parse([]byte(text))
	if err != nil {
		return jsonvalue.Value{}, fmt.Errorf("dispatch: stored response: %w", err)
	}
	return v, nil
}

// journalStore is the journal's view of the store during one activation. When the
// durable engine supplies a checkpoint, every transaction of the journal runs inside
// it, so the lease is checked on each write; reads go straight to the store.
type journalStore struct {
	base       journal.Store
	checkpoint func(ctx context.Context, fn func(ctx context.Context, tx any) error) error
}

// operationPage is the page size of the reads of an activation's journal rows.
const operationPage = 512

// InTx implements [journal.Store].
func (s journalStore) InTx(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx store.Tx) error) error {
	if s.checkpoint == nil {
		return s.base.InTx(ctx, org, fn)
	}
	return s.checkpoint(ctx, func(ctx context.Context, tx any) error {
		t, ok := tx.(store.Tx)
		if !ok {
			return fmt.Errorf("dispatch: checkpoint transaction is %T, not a store transaction", tx)
		}
		return fn(ctx, t)
	})
}

// ListExternalOperations implements [journal.Store].
func (s journalStore) ListExternalOperations(ctx context.Context, org orgscope.Org, node store.NodeExecutionID, fromItem int64, fromID store.ExternalOperationID, limit int) ([]store.ExternalOperation, error) {
	return s.base.ListExternalOperations(ctx, org, node, fromItem, fromID, limit)
}

// effectProvider performs the sends of a journal. Results are kept per item; a
// send whose outcome is unknown returns its error, which the journal classifies.
type effectProvider struct {
	n    *HTTPNode
	req  request
	cred Credential

	mu      sync.Mutex
	results []*jsonvalue.Value
}

// Send performs one item. The journal calls it outside every transaction.
func (p *effectProvider) Send(ctx context.Context, a *journal.Attempt) (journal.Reply, error) {
	i := int(a.Operation.ItemIndex)
	items, err := p.n.sendOnce(ctx, p.req, p.cred, a.Operation.CorrelationKey)
	if err != nil {
		if errors.Is(err, ErrUnknownOutcome) {
			return journal.Reply{}, err
		}
		return journal.Reply{Outcome: journal.StateFailed}, nil
	}
	v := jsonvalue.Null()
	if len(items) > 0 {
		v = items[0]
	}
	// The response is kept by the journal, so that a later run reads it instead of sending again.
	text, err := jsonvalue.Marshal(v)
	if err != nil {
		return journal.Reply{}, fmt.Errorf("dispatch: encode response: %w", err)
	}
	p.mu.Lock()
	p.results[i] = &v
	p.mu.Unlock()
	return journal.Reply{Outcome: journal.StateSucceeded, Response: string(text)}, nil
}

// storeIdentities converts the interpreter's text identities of a call to the
// store's ids; the rows of the journal refer to these rows.
func storeIdentities(call execution.NodeCall) (store.ExecutionID, store.TokenID, store.NodeExecutionID, error) {
	e, err := id.Parse(string(call.Execution))
	if err != nil {
		return store.ExecutionID{}, store.TokenID{}, store.NodeExecutionID{}, fmt.Errorf("dispatch: execution identity: %w", err)
	}
	t, err := id.Parse(string(call.Token))
	if err != nil {
		return store.ExecutionID{}, store.TokenID{}, store.NodeExecutionID{}, fmt.Errorf("dispatch: token identity: %w", err)
	}
	n, err := id.Parse(string(call.NodeExecution))
	if err != nil {
		return store.ExecutionID{}, store.TokenID{}, store.NodeExecutionID{}, fmt.Errorf("dispatch: node execution identity: %w", err)
	}
	return store.ExecutionID(e), store.TokenID(t), store.NodeExecutionID(n), nil
}

// Lookup is not offered: the node's contracts are not reconcilable.
func (p *effectProvider) Lookup(context.Context, *journal.Attempt) (journal.Reply, error) {
	return journal.Reply{}, errors.New("dispatch: reconciliation is not offered by orcon.http@1")
}

var _ journal.Provider = (*effectProvider)(nil)

// Router is the node runner of an engine: it hands each activation to the runner
// of its node type. A type without a runner fails the node.
type Router struct {
	runners map[string]execution.NodeRunner
}

// NewRouter returns a router over the given runners, keyed by node type.
func NewRouter(runners map[string]execution.NodeRunner) *Router {
	return &Router{runners: runners}
}

// Execute implements [execution.NodeRunner].
func (r *Router) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	runner, ok := r.runners[call.Node.Type]
	if !ok {
		return execution.NodeOutput{}, fmt.Errorf("dispatch: no runner for node type %q", call.Node.Type)
	}
	return runner.Execute(ctx, call)
}

var (
	_ execution.NodeRunner = (*HTTPNode)(nil)
	_ execution.NodeRunner = (*Router)(nil)
)
