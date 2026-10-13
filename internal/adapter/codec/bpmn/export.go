// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import (
	"strconv"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Export writes a flow as BPMN 2.0 XML with BPMNDI (ADR-0007 section 8). The
// flow must pass [flow.Validate]; otherwise the error is a
// [*flow.ValidationError]. The output is deterministic: the same flow always
// gives the same bytes.
//
// Export fails with an [*UnsupportedError] for nodes that need waits, events,
// call activities or multi-instance (see the package comment), and with an
// [*UnrepresentableError] for a string XML 1.0 cannot carry.
func Export(doc *flow.Document) ([]byte, error) {
	if report := flow.Validate(doc); report.HasErrors() {
		return nil, &flow.ValidationError{Report: report}
	}
	root := doc.Value()
	nodes, _ := root.Get(memberNodes)
	if err := checkScope(nodes); err != nil {
		return nil, err
	}
	edges, _ := root.Get(memberEdges)

	w := &xmlWriter{}
	w.header()
	flowID := stringMember(root, memberID)
	w.where = "/" + memberID
	w.start("bpmn:definitions", []attr{
		{"xmlns:bpmn", nsModel},
		{"xmlns:bpmndi", nsBPMNDI},
		{"xmlns:dc", nsDC},
		{"xmlns:di", nsDI},
		{"xmlns:xsi", nsXSI},
		{"xmlns:orcon", NamespaceOrcon},
		{"id", "definitions"},
		{"targetNamespace", "urn:orcon:flow:" + flowID},
		{"exporter", "Orcon"},
		{"orcon:format", stringMember(root, memberFormat)},
	}, false)

	w.where = "/" + memberName
	w.start("bpmn:process", []attr{
		{"id", idFlow + flowID},
		{"name", stringMember(root, memberName)},
		{"isExecutable", "false"},
	}, false)
	if desc, ok := root.Get(memberDescription); ok {
		s, _ := desc.Str()
		w.where = "/" + memberDescription
		w.leaf("bpmn:documentation", nil, s)
	}
	writeProcessExtensions(w, root)
	layoutNodes, layoutEdges := writeNodes(w, nodes, edges)
	writeEdges(w, edges)
	w.end("bpmn:process")

	writeDiagram(w, idFlow+flowID, layoutNodes, layoutEdges)
	w.end("bpmn:definitions")
	if w.err != nil {
		return nil, w.err
	}
	w.buf.WriteByte('\n')
	return w.buf.Bytes(), nil
}

// stringMember returns the string held by an object member, or "".
func stringMember(v jsonvalue.Value, key string) string {
	m, ok := v.Get(key)
	if !ok {
		return ""
	}
	s, _ := m.Str()
	return s
}

// writeProcessExtensions writes the flow members that have no native place.
func writeProcessExtensions(w *xmlWriter, root jsonvalue.Value) {
	var rest []jsonvalue.Member
	for _, m := range root.Members() {
		switch m.Key {
		case memberFormat, memberID, memberName, memberDescription, memberNodes, memberEdges:
		default:
			rest = append(rest, m)
		}
	}
	writeExtensions(w, rest, "")
}

// writeExtensions writes members as typed extension elements inside
// extensionElements; nothing is written when there are none.
func writeExtensions(w *xmlWriter, members []jsonvalue.Member, base string) {
	if len(members) == 0 {
		return
	}
	w.start("bpmn:extensionElements", nil, false)
	for _, m := range members {
		w.value(m.Key, true, m.Value, pointer(base, m.Key))
	}
	w.end("bpmn:extensionElements")
}

func writeNodes(w *xmlWriter, nodes, edges jsonvalue.Value) ([]layoutNode, []layoutEdge) {
	defaults := defaultEdges(edges)
	items := nodes.Items()
	layoutNodes := make([]layoutNode, 0, len(items))
	for i, n := range items {
		base := pointer("/"+memberNodes, strconv.Itoa(i))
		id := stringMember(n, memberID)
		element := stringMember(n, memberElement)
		layoutNodes = append(layoutNodes, layoutNode{id: id, element: flow.Element(element)})

		attrs := []attr{{"id", idNode + id}, {"name", stringMember(n, memberName)}}
		if def, ok := defaults[id]; ok && hasDefaultAttribute(flow.Element(element)) {
			attrs = append(attrs, attr{"default", idEdge + def})
		}
		if routing, ok := n.Get(memberRouting); ok {
			s, _ := routing.Str()
			attrs = append(attrs, attr{"orcon:itemRouting", s})
		}
		if _, ok := n.Get(memberLoop); ok {
			attrs = append(attrs, attr{"orcon:loopControl", "true"})
		}

		var rest []jsonvalue.Member
		for _, m := range n.Members() {
			switch m.Key {
			case memberID, memberName, memberElement, memberRouting:
			default:
				rest = append(rest, m)
			}
		}
		w.where = pointer(base, memberName)
		w.start("bpmn:"+element, attrs, false)
		writeExtensions(w, rest, base)
		if activatesOnFirst(flow.Element(element), n) {
			w.leaf("bpmn:activationCondition", nil, "1")
		}
		w.end("bpmn:" + element)
	}
	var layoutEdges []layoutEdge
	for _, e := range edges.Items() {
		from, _ := e.Get(memberFrom)
		to, _ := e.Get(memberTo)
		kind := stringMember(e, "kind")
		layoutEdges = append(layoutEdges, layoutEdge{
			id:   stringMember(e, memberID),
			from: stringMember(from, memberNode),
			to:   stringMember(to, memberNode),
			back: kind == string(flow.EdgeBack),
		})
	}
	return layoutNodes, layoutEdges
}

func writeEdges(w *xmlWriter, edges jsonvalue.Value) {
	for i, e := range edges.Items() {
		base := pointer("/"+memberEdges, strconv.Itoa(i))
		from, _ := e.Get(memberFrom)
		to, _ := e.Get(memberTo)

		var rest []jsonvalue.Member
		if port, ok := from.Get(memberPort); ok {
			rest = append(rest, jsonvalue.Member{Key: memberPort, Value: port})
		}
		for _, m := range e.Members() {
			switch m.Key {
			case memberID, memberFrom, memberTo, memberCondition:
			default:
				rest = append(rest, m)
			}
		}
		w.where = pointer(base, memberID)
		w.start("bpmn:sequenceFlow", []attr{
			{"id", idEdge + stringMember(e, memberID)},
			{"sourceRef", idNode + stringMember(from, memberNode)},
			{"targetRef", idNode + stringMember(to, memberNode)},
		}, false)
		writeExtensions(w, rest, base)
		if cond, ok := e.Get(memberCondition); ok {
			s, _ := cond.Str()
			w.where = pointer(base, memberCondition)
			w.leaf("bpmn:conditionExpression", []attr{
				{"xsi:type", "bpmn:tFormalExpression"},
				{"language", ExpressionLanguage},
			}, s)
		}
		w.end("bpmn:sequenceFlow")
	}
}

// defaultEdges maps a source node to its default edge when it has exactly one.
func defaultEdges(edges jsonvalue.Value) map[string]string {
	count := map[string]int{}
	last := map[string]string{}
	for _, e := range edges.Items() {
		if d, ok := e.Get(memberDefault); ok {
			if b, _ := d.Bool(); b {
				from, _ := e.Get(memberFrom)
				src := stringMember(from, memberNode)
				count[src]++
				last[src] = stringMember(e, memberID)
			}
		}
	}
	out := map[string]string{}
	for src, n := range count {
		if n == 1 {
			out[src] = last[src]
		}
	}
	return out
}

// hasDefaultAttribute reports whether BPMN allows a default flow on the element.
func hasDefaultAttribute(e flow.Element) bool {
	switch e {
	case flow.ElementExclusiveGateway, flow.ElementInclusiveGateway, flow.ElementComplexGateway,
		flow.ElementTask, flow.ElementServiceTask, flow.ElementScriptTask, flow.ElementUserTask,
		flow.ElementReceiveTask, flow.ElementSendTask:
		return true
	case flow.ElementStartEvent, flow.ElementEndEvent, flow.ElementCallActivity,
		flow.ElementParallelGateway, flow.ElementEventBasedGateway,
		flow.ElementIntermediateCatchEvent, flow.ElementIntermediateThrowEvent,
		flow.ElementBoundaryEvent:
		return false
	default:
		return false
	}
}

// activatesOnFirst reports whether the node is a complex gateway whose join
// fires on the first arrival, which BPMN states as an activation condition of 1.
func activatesOnFirst(e flow.Element, n jsonvalue.Value) bool {
	if e != flow.ElementComplexGateway {
		return false
	}
	join, ok := n.Get(memberJoin)
	if !ok {
		return false
	}
	return stringMember(join, "mode") == string(flow.JoinFirst)
}

func writeDiagram(w *xmlWriter, processID string, nodes []layoutNode, edges []layoutEdge) {
	d := layoutGraph(nodes, edges)
	w.where = ""
	w.start("bpmndi:BPMNDiagram", []attr{{"id", "diagram"}}, false)
	w.start("bpmndi:BPMNPlane", []attr{{"id", "plane"}, {"bpmnElement", processID}}, false)
	for _, n := range nodes {
		b := d.nodes[n.id]
		w.start("bpmndi:BPMNShape", []attr{{"id", "shape_" + idNode + n.id}, {"bpmnElement", idNode + n.id}}, false)
		w.start("dc:Bounds", []attr{
			{"x", strconv.Itoa(b.x)}, {"y", strconv.Itoa(b.y)},
			{"width", strconv.Itoa(b.w)}, {"height", strconv.Itoa(b.h)},
		}, true)
		w.end("bpmndi:BPMNShape")
	}
	for _, e := range edges {
		pts := d.edges[e.id]
		w.start("bpmndi:BPMNEdge", []attr{{"id", "shape_" + idEdge + e.id}, {"bpmnElement", idEdge + e.id}}, false)
		for _, p := range pts {
			w.start("di:waypoint", []attr{{"x", strconv.Itoa(p.x)}, {"y", strconv.Itoa(p.y)}}, true)
		}
		w.end("bpmndi:BPMNEdge")
	}
	w.end("bpmndi:BPMNPlane")
	w.end("bpmndi:BPMNDiagram")
}
