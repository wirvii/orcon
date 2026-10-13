// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"fmt"
	"slices"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// FindingCode classifies a finding of the node mapping.
type FindingCode string

// Finding codes (UPPER_SNAKE: they appear in import reports).
const (
	// FindUnmappedType: no declared mapping for the n8n node type.
	FindUnmappedType FindingCode = "UNMAPPED_TYPE"
	// FindUnmappedTypeVersion: the type is mapped but not this typeVersion.
	FindUnmappedTypeVersion FindingCode = "UNMAPPED_TYPE_VERSION"
	// FindSelectorExpression: the resource or operation is an expression.
	FindSelectorExpression FindingCode = "SELECTOR_EXPRESSION"
	// FindUnmappedOperation: the resource and operation pair is not in the
	// parameter map of the catalog.
	FindUnmappedOperation FindingCode = "UNMAPPED_OPERATION"
	// FindUnmappedCredential: no Orcon credential type declared for the n8n one.
	FindUnmappedCredential FindingCode = "UNMAPPED_CREDENTIAL" //nolint:gosec // G101: a finding code, not a credential
	// FindPlannedOnly: the connector is planned and not installed.
	FindPlannedOnly FindingCode = "PLANNED_ONLY"
	// FindUnsupportedCore: a core node with no counterpart yet.
	FindUnsupportedCore FindingCode = "UNSUPPORTED_CORE"
	// FindReviewRequired: a person must review the node before the flow runs.
	FindReviewRequired FindingCode = "REVIEW_REQUIRED"
)

// Finding is one thing the mapping of a node reports. Field is the path of
// the offending member in the n8n node, such as "parameters.operation".
// Nothing is approximated: a finding means the thing was not mapped.
type Finding struct {
	Code    FindingCode
	NodeID  string
	Node    string
	Field   string
	Message string
}

// Safety is the safety level the importer can state for an HTTP node.
type Safety string

// Safety levels (UPPER_SNAKE).
const (
	// SafetyRead: a GET or HEAD request, READ purity.
	SafetyRead Safety = "READ"
	// SafetyUnsafe: a write without a declared idempotency key.
	SafetyUnsafe Safety = "UNSAFE"
)

// MappedCredential is a credential reference after mapping. It holds a type
// and a name and never a value.
type MappedCredential struct {
	N8nType string
	Name    string
	// OrconType is empty when the catalog declares no counterpart.
	OrconType string
}

// Mapped is the outcome of mapping one n8n node. When OrconType and Construct
// are both empty the node was not mapped and Findings say why.
type Mapped struct {
	// OrconType is the Orcon node type of a node that maps to one type.
	OrconType string
	// Construct names the Orcon construct of a node that maps to something
	// other than a single node type (a join block, a durable wait, an editor
	// note, a family of nodes).
	Construct string
	// Operation is the selected operation, resource.verb, for connector nodes.
	Operation string
	// Safety is set for HTTP Request nodes.
	Safety Safety
	// Ext is the ext member of the Orcon node: {"orcon.import": {...}}.
	Ext         jsonvalue.Value
	Credentials []MappedCredential
	Findings    []Finding
}

// Mapped reports whether the node maps to something in Orcon.
func (m *Mapped) Mapped() bool { return m.OrconType != "" || m.Construct != "" }

// ReviewRequired reports whether the node waits for a person's review.
func (m *Mapped) ReviewRequired() bool {
	for _, f := range m.Findings {
		if f.Code == FindReviewRequired {
			return true
		}
	}
	return false
}

// MapNode maps one n8n node. It never substitutes silently: whatever it
// cannot map is reported with the node and the field, and the node is left
// unmapped (ADR-0037 section 6, ADR-0034 section 8).
//
// Core nodes map at the type level; the translation of their parameters
// belongs to the cut that implements them. Connector nodes map through the
// catalog: the typeVersion must be declared, and the n8n resource and
// operation pair becomes the single reserved parameter operation,
// resource.verb (SPEC-003 amendment 8, S-7).
func MapNode(n *Node, cat *Catalog) Mapped {
	var m Mapped
	report := func(code FindingCode, field, msg string) {
		m.Findings = append(m.Findings, Finding{Code: code, NodeID: n.ID, Node: n.Name, Field: field, Message: msg})
	}
	typ, _ := normalizeType(n.Type)
	if _, err := jsonvalue.Number(n.TypeVersion); err != nil {
		report(FindUnmappedTypeVersion, "typeVersion", "the typeVersion is not a number, so it cannot be kept")
		return m
	}
	switch {
	case IsAIType(typ):
		report(FindUnmappedType, "type", "AI nodes are mapped by the AI design (SPEC-006)")
		return m
	case !IsBuiltin(typ):
		report(FindUnmappedType, "type", "community nodes need an installed converted package (ADR-0035)")
		return m
	}
	if row, ok := LookupCore(typ); ok {
		mapCore(n, typ, row, &m, report)
	} else {
		mapConnector(n, typ, cat, &m, report)
	}
	m.Credentials = mapCredentials(n, cat, report)
	if m.Mapped() {
		m.Ext = importExt(n, m.reviewReasons())
	}
	return m
}

func mapCore(n *Node, typ string, row CoreNode, m *Mapped, report func(FindingCode, string, string)) {
	if !row.Mapped() {
		report(FindUnsupportedCore, "type", row.Note)
		return
	}
	short := strings.TrimPrefix(typ, basePrefix)
	switch {
	case row.Orcon == "orcon.<name>@1":
		m.OrconType = "orcon." + short + "@1"
	case strings.HasPrefix(row.Orcon, "orcon.") && strings.HasSuffix(row.Orcon, "@1") && !strings.ContainsAny(row.Orcon, "*<, "):
		m.OrconType = row.Orcon
	default:
		m.Construct = row.Orcon
	}
	if IsHTTPRequest(typ) {
		mapHTTP(n, m, report)
	}
}

// mapHTTP states the safety of an HTTP Request node from its method
// (SPEC-003 Annex B): GET and HEAD are reads; every other method, and an
// expression that may be one, is UNSAFE and waits for review.
func mapHTTP(n *Node, m *Mapped, report func(FindingCode, string, string)) {
	field, method := "parameters.method", paramString(n, "method")
	if _, ok := n.Parameters.Get("method"); !ok {
		if _, legacy := n.Parameters.Get("requestMethod"); legacy {
			field, method = "parameters.requestMethod", paramString(n, "requestMethod")
		}
	}
	switch method = strings.TrimSpace(method); {
	case method == "":
		m.Safety = SafetyRead // n8n sends GET when the method is left out
	case strings.HasPrefix(method, "="):
		m.Safety = SafetyUnsafe
		report(FindReviewRequired, field, flow.ReasonHTTPMethodExpression+": the method is an expression, so the request is UNSAFE until a person reviews it")
	case strings.EqualFold(method, "GET") || strings.EqualFold(method, "HEAD"):
		m.Safety = SafetyRead
	default:
		m.Safety = SafetyUnsafe
		report(FindReviewRequired, field, flow.ReasonHTTPWrite+": "+strings.ToUpper(method)+" changes remote state without an idempotency key, so the node is UNSAFE until a person reviews it")
	}
}

func paramString(n *Node, key string) string {
	p, ok := n.Parameters.Get(key)
	if !ok {
		return ""
	}
	s, _ := p.Str()
	return s
}

func mapConnector(n *Node, typ string, cat *Catalog, m *Mapped, report func(FindingCode, string, string)) {
	if !cat.Has(typ) {
		report(FindUnmappedType, "type", "no connector declares "+typ)
		return
	}
	if !cat.HasVersion(typ, n.TypeVersion) {
		report(FindUnmappedTypeVersion, "typeVersion", "no mapping declared for typeVersion "+n.TypeVersion)
		return
	}
	k, _ := cat.Connector(typ)
	unit, sel := UnitOf(n)
	unit.Type = typ
	if sel.Expression {
		field := "parameters.operation"
		if isExpression(n, "resource") {
			field = "parameters.resource"
		}
		report(FindSelectorExpression, field, "the resource or operation is an expression, so the operation cannot be chosen; a person must choose it")
		return
	}
	info, ok := cat.LookupUnit(unit)
	if !ok {
		report(FindUnmappedOperation, "parameters.operation",
			fmt.Sprintf("the pair resource %q, operation %q is not in the parameter map", sel.Resource, sel.Operation))
		return
	}
	if k.OrconType == "" {
		report(FindPlannedOnly, "type", "connector "+k.ID+" is planned and not installed")
		return
	}
	m.OrconType = k.OrconType
	m.Operation = info.Operation
	if info.Unsafe {
		m.Safety = SafetyUnsafe
		report(FindReviewRequired, "parameters.operation",
			flow.ReasonUnsafeOperation+": the contract of "+info.Operation+" has safety level UNSAFE, so the node waits for a person's review")
	}
}

func isExpression(n *Node, key string) bool {
	p, ok := n.Parameters.Get(key)
	if !ok {
		return false
	}
	s, _ := p.Str()
	return strings.HasPrefix(s, "=")
}

func mapCredentials(n *Node, cat *Catalog, report func(FindingCode, string, string)) []MappedCredential {
	var out []MappedCredential
	for _, ref := range n.Credentials {
		mc := MappedCredential{N8nType: ref.Type, Name: ref.Name}
		if o, ok := cat.Credential(ref.Type); ok {
			mc.OrconType = o
		} else {
			report(FindUnmappedCredential, "credentials."+ref.Type, "no Orcon credential type declared for "+ref.Type+"; the credential is kept by reference and must be created again")
		}
		out = append(out, mc)
	}
	return out
}

// reviewReasons lists the review reasons among the findings, once each.
func (m *Mapped) reviewReasons() []string {
	var out []string
	for _, f := range m.Findings {
		if f.Code != FindReviewRequired {
			continue
		}
		reason, _, _ := strings.Cut(f.Message, ":")
		if !slices.Contains(out, reason) {
			out = append(out, reason)
		}
	}
	return out
}

// importExt builds the ext member of an imported node: the closed object of
// ADR-0034 section 8 (amendment 9 of SPEC-003).
func importExt(n *Node, reasons []string) jsonvalue.Value {
	version, _ := jsonvalue.Number(n.TypeVersion) // checked by MapNode
	members := []jsonvalue.Member{
		{Key: "n8nType", Value: jsonvalue.String(n.Type)},
		{Key: "n8nTypeVersion", Value: version},
	}
	if n.ID != "" {
		members = append(members, jsonvalue.Member{Key: "n8nNodeId", Value: jsonvalue.String(n.ID)})
	}
	if len(reasons) > 0 {
		rs := make([]jsonvalue.Value, len(reasons))
		for i, r := range reasons {
			rs[i] = jsonvalue.String(r)
		}
		members = append(members,
			jsonvalue.Member{Key: "reviewStatus", Value: jsonvalue.String(flow.ReviewNeeds)},
			jsonvalue.Member{Key: "reviewReasons", Value: jsonvalue.Array(rs...)})
	}
	inner, _ := jsonvalue.Object(members...)
	ext, _ := jsonvalue.Object(jsonvalue.Member{Key: flow.ImportExtKey, Value: inner})
	return ext
}
