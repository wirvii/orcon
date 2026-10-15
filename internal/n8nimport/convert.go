// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Finding codes added by the workflow converter (UPPER_SNAKE, as the report
// shows them).
const (
	// FindUnsupportedParameter: a parameter the converter does not translate.
	FindUnsupportedParameter FindingCode = "UNSUPPORTED_PARAMETER"
	// FindUnsupportedConnection: a connection the converter does not translate.
	FindUnsupportedConnection FindingCode = "UNSUPPORTED_CONNECTION"
	// FindDanglingConnection: a connection that names a node the workflow does not have.
	FindDanglingConnection FindingCode = "DANGLING_CONNECTION"
	// FindDisabled: the node was switched off in n8n; the flow keeps it disabled.
	FindDisabled FindingCode = "DISABLED"
	// FindNotOnMain: the Orcon counterpart of a core node is not on main yet.
	FindNotOnMain FindingCode = "NOT_ON_MAIN"
)

// Outcome says what the converter did with one n8n node.
type Outcome string

// Outcomes of a node (UPPER_SNAKE).
const (
	// OutcomeConverted: the node runs as the Orcon node it names.
	OutcomeConverted Outcome = "CONVERTED"
	// OutcomeAdapted: the node runs; the report states a difference.
	OutcomeAdapted Outcome = "ADAPTED"
	// OutcomeNeedsReview: the node runs only after a person reviews it.
	OutcomeNeedsReview Outcome = "NEEDS_REVIEW"
	// OutcomeUnsupported: the node is kept as orcon.unsupported@1 with the
	// original n8n node in its ext. The flow cannot run until it is rebuilt.
	OutcomeUnsupported Outcome = "UNSUPPORTED"
)

// unsupportedType is the node type that keeps an n8n node Orcon cannot run.
const unsupportedType = "orcon.unsupported@1"

// NodeOutcome is the report entry of one n8n node.
type NodeOutcome struct {
	Name        string
	ID          flow.NodeID
	N8nType     string
	TypeVersion string
	Outcome     Outcome
	// OrconType is the Orcon node type the node became; empty when it is
	// unsupported.
	OrconType string
	Findings  []Finding
}

// Conversion is the result of converting one n8n workflow: the flow document
// and one report entry per node. Nothing is dropped silently; whatever the
// converter could not carry over is in the report (SPEC-003 R-IMP-01, D7b).
type Conversion struct {
	// Name is the workflow name as n8n wrote it.
	Name     string
	Document flow.Document
	// Nodes has one entry per n8n node, in the order of the workflow. Sticky
	// notes are editor data and have no entry.
	Nodes []NodeOutcome
	// Findings holds the findings that belong to no single node: the sticky
	// notes the converter had to trim.
	Findings []Finding
	// Problems lists what flow validation says about the produced document.
	// A problem is a converter defect, except the unreachable sub-nodes of AI
	// nodes, whose connections the report already states.
	Problems []string
}

// FlowJSON returns the flow document as JSON in the member order of the
// format; it is the same encoding the flow codec writes.
func (c *Conversion) FlowJSON() ([]byte, error) {
	data, err := jsonvalue.Marshal(c.Document.Value())
	if err != nil {
		return nil, fmt.Errorf("n8nimport: encode flow: %w", err)
	}
	return data, nil
}

// ConvertWorkflow converts an n8n workflow export (or a template-library
// wrapper around one) into a flow document with the given id. The catalog
// tells which connector nodes are installed; nil means none. The error is
// only for input that is not a workflow at all.
func ConvertWorkflow(data []byte, id flow.ID, cat *Catalog) (*Conversion, error) {
	wf, err := parseWorkflow(data)
	if err != nil {
		return nil, err
	}
	if cat == nil {
		cat = Installed()
	}
	c := &converter{wf: wf, cat: cat, ids: map[string]flow.NodeID{}, built: map[string]*built{}}
	return c.run(id)
}

// workflow is the part of an n8n export the converter reads.
type workflow struct {
	name        string
	timezone    string
	nodes       []Node
	connections jsonvalue.Value
}

// parseWorkflow reads an n8n workflow export. It accepts the export as n8n
// writes it, the same under a "workflow" wrapper, and the template library's
// two-level wrapper.
func parseWorkflow(data []byte) (workflow, error) {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return workflow{}, fmt.Errorf("n8nimport: parse workflow: %w", err)
	}
	root := v
	if w, ok := v.Get("workflow"); ok && w.Kind() == jsonvalue.KindObject {
		root = w
		if inner, ok := w.Get("workflow"); ok && inner.Kind() == jsonvalue.KindObject {
			root = inner
		}
	}
	nodes, ok := root.Get("nodes")
	if !ok || nodes.Kind() != jsonvalue.KindArray {
		return workflow{}, ErrNoNodes
	}
	wf := workflow{name: strValue(root, "name")}
	for i, item := range nodes.Items() {
		n, err := nodeFrom(item)
		if err != nil {
			return workflow{}, fmt.Errorf("n8nimport: node %d: %w", i, err)
		}
		wf.nodes = append(wf.nodes, n)
	}
	if s, ok := root.Get("settings"); ok {
		wf.timezone = strValue(s, "timezone")
	}
	if c, ok := root.Get("connections"); ok && c.Kind() == jsonvalue.KindObject {
		wf.connections = c
	}
	return wf, nil
}

// built is one converted n8n node on its way into the document.
type built struct {
	node flow.Node
	// ports is the Orcon port of every n8n output, by output index. Nil means
	// every output is the "main" port.
	ports []string
	// routed marks an item-routing node: its "main" port is the default edge.
	routed  bool
	outcome NodeOutcome
	// n8nID is the id the workflow gave the node, used in findings.
	n8nID string
	// fanoutNoted is set once the fan-out finding of this node is recorded.
	fanoutNoted bool
}

// addFinding records a finding on the node's report entry.
func (b *built) addFinding(f Finding) {
	f.NodeID = b.n8nID
	b.outcome.Findings = append(b.outcome.Findings, f)
}

type converter struct {
	wf          workflow
	cat         *Catalog
	ids         map[string]flow.NodeID
	built       map[string]*built
	order       []*built
	doc         flow.Document
	out         []Finding
	stickyNotes []StickyNote
}

func (c *converter) run(id flow.ID) (*Conversion, error) {
	if err := c.assignIDs(); err != nil {
		return nil, err
	}
	c.doc = flow.Document{
		Format:      flow.FormatTag,
		ID:          id,
		Name:        c.docName(),
		Description: "Imported from n8n. Review the import report before you publish.",
		Settings:    flow.Settings{BranchMode: flow.BranchSequential},
		Variables:   []flow.Variable{},
	}
	for i := range c.wf.nodes {
		n := &c.wf.nodes[i]
		if IsStickyNote(n.Type) {
			continue
		}
		b := c.convertNode(n)
		c.built[n.Name] = b
		c.order = append(c.order, b)
		c.doc.Nodes = append(c.doc.Nodes, b.node)
	}
	c.collectSticky()
	c.edges()
	c.doc.Editor = c.editor()

	conv := &Conversion{Name: c.wf.name, Document: c.doc, Findings: c.out}
	for _, b := range c.order {
		conv.Nodes = append(conv.Nodes, b.outcome)
	}
	rep := flow.Validate(&c.doc)
	for _, issue := range rep.Errors() {
		conv.Problems = append(conv.Problems, issue.String())
	}
	return conv, nil
}

func (c *converter) docName() string {
	if c.wf.name != "" {
		return c.wf.name
	}
	return "Imported n8n workflow"
}

// idPattern is the form of a node or edge id (flow.IDPattern).
var idPattern = regexp.MustCompile(flow.IDPattern)

// assignIDs gives every n8n node a flow node id: its n8n id when that is a
// valid, unused id, otherwise a numbered one.
func (c *converter) assignIDs() error {
	for i := range c.wf.nodes {
		name := c.wf.nodes[i].Name
		if _, dup := c.ids[name]; dup {
			return fmt.Errorf("n8nimport: two nodes are named %q", name)
		}
		c.ids[name] = ""
	}
	seen := map[string]bool{}
	for i := range c.wf.nodes {
		n := &c.wf.nodes[i]
		candidate := n.ID
		if !idPattern.MatchString(candidate) || seen[candidate] {
			candidate = "node-" + strconv.Itoa(i+1)
			for seen[candidate] {
				candidate += "x"
			}
		}
		seen[candidate] = true
		c.ids[n.Name] = flow.NodeID(candidate)
	}
	return nil
}

// convertNode turns one n8n node into a flow node and its report entry.
func (c *converter) convertNode(n *Node) *built {
	b := &built{n8nID: n.ID}
	var res convResult
	if row, ok := LookupCore(n.Type); ok {
		res = c.convertCore(n, row)
	} else {
		res = c.convertConnector(n)
	}
	b.node = res.node
	if b.node.Ext.IsZero() {
		b.node.Ext = importMembers(n)
	}
	b.ports = res.ports
	b.routed = res.routed
	b.node.ID = c.ids[n.Name]
	b.node.Name = n.Name
	if b.node.Name == "" {
		b.node.Name = string(b.node.ID)
	}
	b.node.Parameters = orEmpty(b.node.Parameters)

	findings := res.findings
	if n.Disabled {
		findings = append(findings, Finding{Code: FindDisabled, NodeID: n.ID, Node: n.Name, Field: "disabled",
			Message: "the node is switched off in n8n; the flow keeps it switched off"})
		t := true
		b.node.Disabled = &t
	}
	outcome := res.outcome
	if outcome == "" {
		outcome = outcomeOfFindings(findings)
	}
	b.outcome = NodeOutcome{
		Name: n.Name, ID: b.node.ID, N8nType: n.Type, TypeVersion: n.TypeVersion,
		Outcome: outcome, OrconType: res.orconType, Findings: findings,
	}
	if outcome == OutcomeUnsupported {
		b.outcome.OrconType = ""
	}
	return b
}

// outcomeOfFindings is the outcome of a node that runs: needs review when a
// person must look at it first, adapted when a difference is stated.
func outcomeOfFindings(findings []Finding) Outcome {
	outcome := OutcomeConverted
	for _, f := range findings {
		if f.Code == FindReviewRequired {
			return OutcomeNeedsReview
		}
		if f.Code == FindAdapted || f.Code == FindDisabled {
			outcome = OutcomeAdapted
		}
	}
	return outcome
}

// orEmpty replaces a zero parameters value with an empty object.
func orEmpty(v jsonvalue.Value) jsonvalue.Value {
	if v.IsZero() {
		return emptyObject()
	}
	return v
}

// convResult is what a converter function returns for one node.
type convResult struct {
	node      flow.Node
	orconType string
	outcome   Outcome
	ports     []string
	routed    bool
	findings  []Finding
}

// unsupported keeps the node as orcon.unsupported@1 with the original n8n
// node in its ext. Its findings say why.
func (c *converter) unsupported(n *Node, findings []Finding, element flow.Element) convResult {
	// The original node is kept without its secrets; each replaced field is a
	// finding of its own.
	clean, secretFindings := redactNode(n)
	if len(findings) == 0 && len(secretFindings) == 0 {
		findings = []Finding{{Code: FindUnmappedType, NodeID: n.ID, Node: n.Name, Field: "type",
			Message: "no Orcon counterpart is available for " + n.Type}}
	}
	ext := importMembers(n)
	if !clean.IsZero() {
		inner, _ := ext.Get(flow.ImportExtKey)
		ext = ext.With(flow.ImportExtKey, inner.With("original", clean))
	}
	return convResult{
		node: flow.Node{Element: element, Type: unsupportedType, Parameters: emptyObject(),
			Ext: ext},
		outcome:  OutcomeUnsupported,
		findings: append(findings, secretFindings...),
	}
}

// convertConnector maps a node that is not a core type through the catalog.
// It keeps the same answers MapNode gives, so the coverage harness and the
// converter cannot disagree.
func (c *converter) convertConnector(n *Node) convResult {
	m := MapNode(n, c.cat)
	if m.Mapped() && m.OrconType != "" {
		params := emptyObject()
		if m.Operation != "" {
			params, _ = jsonvalue.Object(jsonvalue.Member{Key: "operation", Value: jsonvalue.String(m.Operation)})
		}
		return convResult{
			node:      flow.Node{Element: flow.ElementServiceTask, Type: m.OrconType, Parameters: params, Ext: m.Ext},
			orconType: m.OrconType,
			findings:  m.Findings,
		}
	}
	return c.unsupported(n, m.Findings, elementFor(n))
}

// elementFor is the BPMN class of an unsupported node: a trigger starts the
// flow, anything else is a task.
func elementFor(n *Node) flow.Element {
	if strings.HasSuffix(strings.TrimPrefix(n.Type, basePrefix), "Trigger") {
		return flow.ElementStartEvent
	}
	return flow.ElementTask
}

// importMembers builds ext["orcon.import"] for a node: the n8n type, its
// version and its id. [converter.unsupported] adds the original node, the only
// node type that keeps one.
func importMembers(n *Node) jsonvalue.Value {
	version, _ := jsonvalue.Number(n.TypeVersion)
	members := []jsonvalue.Member{
		{Key: "n8nType", Value: jsonvalue.String(n.Type)},
		{Key: "n8nTypeVersion", Value: version},
	}
	if n.ID != "" {
		members = append(members, jsonvalue.Member{Key: "n8nNodeId", Value: jsonvalue.String(n.ID)})
	}
	inner, _ := jsonvalue.Object(members...)
	ext, _ := jsonvalue.Object(jsonvalue.Member{Key: flow.ImportExtKey, Value: inner})
	return ext
}

func emptyObject() jsonvalue.Value {
	v, _ := jsonvalue.Object()
	return v
}

// edges wires the n8n connections into flow edges, in the order the workflow
// lists them. Each n8n output maps to an Orcon port (see the core converters).
func (c *converter) edges() {
	n := 0
	for _, m := range c.wf.connections.Members() {
		src, ok := c.built[m.Key]
		if !ok {
			continue
		}
		for _, kind := range m.Value.Members() {
			for k, outs := range kind.Value.Items() {
				for _, target := range outs.Items() {
					e, ok := c.edgeFor(src, m.Key, kind.Key, k, target)
					if !ok {
						continue
					}
					n++
					e.ID = flow.EdgeID("e" + strconv.Itoa(n))
					e.Order = n
					c.doc.Edges = append(c.doc.Edges, e)
				}
			}
		}
	}
}

// edgeFor turns one n8n connection from output k of a node into a flow edge.
// A connection that cannot be carried is reported on the source node.
func (c *converter) edgeFor(src *built, srcName, kind string, k int, target jsonvalue.Value) (flow.Edge, bool) {
	to := strValue(target, "node")
	if kind != "main" {
		src.addFinding(Finding{Code: FindUnsupportedConnection, Node: srcName, Field: "connections." + kind,
			Message: "the " + kind + " connection is not converted: sub-nodes of AI nodes need the AI design (SPEC-006)"})
		return flow.Edge{}, false
	}
	dst, ok := c.built[to]
	if !ok {
		src.addFinding(Finding{Code: FindDanglingConnection, Node: srcName, Field: "connections.main",
			Message: "the connection goes to " + strconv.Quote(to) + ", which the workflow does not contain"})
		return flow.Edge{}, false
	}
	port, ok := c.portFor(src, srcName, k)
	if !ok {
		return flow.Edge{}, false
	}
	e := flow.Edge{
		From: flow.Endpoint{Node: src.node.ID, Port: port},
		To:   dst.node.ID,
		Kind: flow.EdgeSequence,
	}
	if src.routed && port == "main" {
		t := true
		e.Default = &t
	}
	return e, true
}

// portFor returns the Orcon port of n8n output k. It reports false, after
// recording why, when the output has no port.
func (c *converter) portFor(src *built, srcName string, k int) (string, bool) {
	if src.ports == nil {
		if k > 0 && !src.fanoutNoted && src.outcome.Outcome == OutcomeUnsupported {
			src.fanoutNoted = true
			src.addFinding(Finding{Code: FindUnsupportedConnection, Node: srcName, Field: "connections.main",
				Message: "this node has more than one output and n8n routes by output; here every output joins the main port, so the branches run together until the node is rebuilt"})
		}
		return "main", true
	}
	if k >= len(src.ports) {
		src.addFinding(Finding{Code: FindUnsupportedConnection, Node: srcName, Field: "connections.main",
			Message: "output " + strconv.Itoa(k) + " has no Orcon port; the connection is not carried"})
		return "", false
	}
	return src.ports[k], true
}

// collectSticky keeps the sticky notes for the editor data and reports the
// ones it had to trim. Sticky notes are not nodes (ADR-0007 section 2).
func (c *converter) collectSticky() {
	for i := range c.wf.nodes {
		n := &c.wf.nodes[i]
		if !IsStickyNote(n.Type) {
			continue
		}
		note, findings := StickyNoteOf(n)
		c.out = append(c.out, findings...)
		if clean, changed := redactText(note.Content); changed {
			note.Content = clean
			c.out = append(c.out, Finding{Code: FindSecretValue, NodeID: n.ID, Node: n.Name, Field: "parameters.content",
				Message: "the note holds a value that looks like a secret and is not kept; it is replaced by " + SecretMarker})
		}
		c.stickyNotes = append(c.stickyNotes, note)
	}
}

// editor builds the editor data: node positions and sticky notes.
func (c *converter) editor() jsonvalue.Value {
	positions := make([]jsonvalue.Member, 0, len(c.order))
	for _, b := range c.order {
		n := c.nodeByName(b.node.Name)
		pos, _ := jsonvalue.Object(
			jsonvalue.Member{Key: "x", Value: jsonvalue.Int(int64(n.Position[0]))},
			jsonvalue.Member{Key: "y", Value: jsonvalue.Int(int64(n.Position[1]))},
		)
		positions = append(positions, jsonvalue.Member{Key: string(b.node.ID), Value: pos})
	}
	nodes, _ := jsonvalue.Object(positions...)
	members := []jsonvalue.Member{{Key: "nodes", Value: nodes}}
	if len(c.stickyNotes) > 0 {
		vals := make([]jsonvalue.Value, 0, len(c.stickyNotes))
		for _, s := range c.stickyNotes {
			vals = append(vals, s.Value())
		}
		members = append(members, jsonvalue.Member{Key: "stickyNotes", Value: jsonvalue.Array(vals...)})
	}
	ed, _ := jsonvalue.Object(members...)
	return ed
}

// nodeByName returns the workflow node of a name; the zero node when there is
// none.
func (c *converter) nodeByName(name string) *Node {
	for i := range c.wf.nodes {
		if c.wf.nodes[i].Name == name {
			return &c.wf.nodes[i]
		}
	}
	return &Node{}
}
