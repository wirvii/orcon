// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// maxXMLDepth bounds the nesting of elements the importer accepts: a flow
// document nests at most [jsonvalue.MaxDepth] levels, and the process, node
// and extension wrappers add a few more.
const maxXMLDepth = jsonvalue.MaxDepth + 16

// Import reads a BPMN 2.0 document produced by [Export] and returns the flow it
// holds. The result has the right shape but may still break the rules of
// [flow.Validate]; callers validate before publishing, as with the JSON codec.
//
// Only documents that declare the namespace urn:orcon:bpmn:1 are accepted.
// Anything else fails with a [*FormatError], and a part this version cannot
// carry with an [*UnsupportedError]. The extension elements are the source of
// truth; the BPMNDI diagram and the other derived attributes are ignored.
func Import(data []byte) (flow.Document, error) {
	root, err := parseTree(data)
	if err != nil {
		return flow.Document{}, err
	}
	v, err := flowValue(root)
	if err != nil {
		return flow.Document{}, err
	}
	doc, err := flow.Read(v)
	if err != nil {
		return flow.Document{}, fmt.Errorf("bpmn: read imported flow: %w", err)
	}
	return doc, nil
}

// element is a parsed XML element: names resolved to namespaces, character
// data kept exactly as written.
type element struct {
	name  xml.Name
	attrs []xml.Attr
	kids  []*element
	text  []byte
}

// attr returns the value of an attribute and whether it is present.
func (e *element) attr(space, local string) (string, bool) {
	for _, a := range e.attrs {
		if a.Name.Space == space && a.Name.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

// onlySpace reports whether the direct text is blank, as in element-only content.
func (e *element) onlySpace() bool { return len(bytes.TrimSpace(e.text)) == 0 }

func parseTree(data []byte) (*element, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = true
	var (
		root  *element
		stack []*element
	)
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, formatErrorf("the document is not well-formed XML: %v", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if len(stack) >= maxXMLDepth {
				return nil, formatErrorf("the document nests deeper than %d levels", maxXMLDepth)
			}
			el := &element{name: t.Name, attrs: slices.Clone(t.Attr)}
			switch {
			case len(stack) > 0:
				parent := stack[len(stack)-1]
				parent.kids = append(parent.kids, el)
			case root != nil:
				return nil, formatErrorf("the document has more than one root element")
			default:
				root = el
			}
			stack = append(stack, el)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				top.text = append(top.text, t...)
			} else if len(bytes.TrimSpace(t)) > 0 {
				return nil, formatErrorf("text outside the root element")
			}
		case xml.Directive:
			return nil, formatErrorf("document type declarations are not accepted")
		case xml.ProcInst:
			if t.Target != "xml" {
				return nil, formatErrorf("processing instructions are not accepted")
			}
		case xml.Comment:
		}
	}
	if root == nil {
		return nil, formatErrorf("the document is empty")
	}
	return root, nil
}

func isModel(e *element, local string) bool {
	return e.name.Space == nsModel && e.name.Local == local
}

// flowValue maps the definitions element to the JSON form of the flow.
func flowValue(defs *element) (jsonvalue.Value, error) {
	if !isModel(defs, "definitions") {
		return jsonvalue.Value{}, formatErrorf("the root element is not a BPMN definitions element")
	}
	format, ok := defs.attr(NamespaceOrcon, memberFormat)
	if !ok {
		return jsonvalue.Value{}, formatErrorf("not an Orcon document: the definitions element has no %s:%s attribute", NamespaceOrcon, memberFormat)
	}
	var process *element
	for _, k := range defs.kids {
		switch {
		case isModel(k, "process"):
			if process != nil {
				return jsonvalue.Value{}, formatErrorf("the document holds more than one process")
			}
			process = k
		case k.name.Space == nsBPMNDI && k.name.Local == "BPMNDiagram":
			// derived layout, ignored
		default:
			return jsonvalue.Value{}, formatErrorf("unexpected element %q in definitions", k.name.Local)
		}
	}
	if process == nil {
		return jsonvalue.Value{}, formatErrorf("the document holds no process")
	}
	return processValue(process, format)
}

func processValue(p *element, format string) (jsonvalue.Value, error) {
	id, err := strippedID(p, idFlow)
	if err != nil {
		return jsonvalue.Value{}, err
	}
	name, ok := p.attr("", "name")
	if !ok {
		return jsonvalue.Value{}, formatErrorf("the process has no name")
	}
	members := []jsonvalue.Member{
		{Key: memberFormat, Value: jsonvalue.String(format)},
		{Key: memberID, Value: jsonvalue.String(id)},
		{Key: memberName, Value: jsonvalue.String(name)},
	}
	taken := map[string]bool{memberFormat: true, memberID: true, memberName: true,
		memberDescription: true, memberNodes: true, memberEdges: true}
	var nodes, edges []jsonvalue.Value
	for _, k := range p.kids {
		switch {
		case isModel(k, "documentation"):
			if len(k.kids) > 0 {
				return jsonvalue.Value{}, formatErrorf("the documentation of the process has child elements")
			}
			members = append(members, jsonvalue.Member{Key: memberDescription, Value: jsonvalue.String(string(k.text))})
		case isModel(k, "extensionElements"):
			ext, err := decodeMembers(k, taken)
			if err != nil {
				return jsonvalue.Value{}, err
			}
			members = append(members, ext...)
		case isModel(k, "sequenceFlow"):
			e, err := edgeValue(k)
			if err != nil {
				return jsonvalue.Value{}, err
			}
			edges = append(edges, e)
		case k.name.Space == nsModel:
			n, err := nodeValue(k, len(nodes))
			if err != nil {
				return jsonvalue.Value{}, err
			}
			nodes = append(nodes, n)
		default:
			return jsonvalue.Value{}, formatErrorf("unexpected element %q in the process", k.name.Local)
		}
	}
	nodesValue := jsonvalue.Array(nodes...)
	if err := checkScope(nodesValue); err != nil {
		return jsonvalue.Value{}, err
	}
	members = append(members,
		jsonvalue.Member{Key: memberNodes, Value: nodesValue},
		jsonvalue.Member{Key: memberEdges, Value: jsonvalue.Array(edges...)})
	v, err := jsonvalue.Object(members...)
	if err != nil {
		return jsonvalue.Value{}, formatErrorf("the process repeats a member: %v", err)
	}
	return v, nil
}

// strippedID returns the id attribute of e without its kind prefix.
func strippedID(e *element, prefix string) (string, error) {
	id, ok := e.attr("", "id")
	if !ok || !strings.HasPrefix(id, prefix) {
		return "", formatErrorf("element %q has no id with the prefix %q", e.name.Local, prefix)
	}
	return strings.TrimPrefix(id, prefix), nil
}

func nodeValue(e *element, index int) (jsonvalue.Value, error) {
	el := flow.Element(e.name.Local)
	if laterElements[el] {
		return jsonvalue.Value{}, &UnsupportedError{
			Pointer: pointer(pointer("", memberNodes), strconv.Itoa(index)) + "/" + memberElement,
			What:    "element " + e.name.Local,
		}
	}
	if !supportedElement(el) {
		return jsonvalue.Value{}, formatErrorf("%q is not a BPMN element of an Orcon flow", e.name.Local)
	}
	id, err := strippedID(e, idNode)
	if err != nil {
		return jsonvalue.Value{}, err
	}
	name, ok := e.attr("", "name")
	if !ok {
		return jsonvalue.Value{}, formatErrorf("node %q has no name", id)
	}
	members := []jsonvalue.Member{
		{Key: memberID, Value: jsonvalue.String(id)},
		{Key: memberName, Value: jsonvalue.String(name)},
		{Key: memberElement, Value: jsonvalue.String(e.name.Local)},
	}
	taken := map[string]bool{memberID: true, memberName: true, memberElement: true, memberRouting: true}
	if routing, ok := e.attr(NamespaceOrcon, "itemRouting"); ok {
		members = append(members, jsonvalue.Member{Key: memberRouting, Value: jsonvalue.String(routing)})
	}
	for _, k := range e.kids {
		switch {
		case isModel(k, "extensionElements"):
			ext, err := decodeMembers(k, taken)
			if err != nil {
				return jsonvalue.Value{}, err
			}
			members = append(members, ext...)
		case isModel(k, "activationCondition"):
			// derived from the join block, ignored
		default:
			return jsonvalue.Value{}, formatErrorf("unexpected element %q in node %q", k.name.Local, id)
		}
	}
	v, err := jsonvalue.Object(members...)
	if err != nil {
		return jsonvalue.Value{}, formatErrorf("node %q repeats a member: %v", id, err)
	}
	return v, nil
}

func edgeValue(e *element) (jsonvalue.Value, error) {
	id, err := strippedID(e, idEdge)
	if err != nil {
		return jsonvalue.Value{}, err
	}
	source, err := strippedRef(e, "sourceRef")
	if err != nil {
		return jsonvalue.Value{}, err
	}
	target, err := strippedRef(e, "targetRef")
	if err != nil {
		return jsonvalue.Value{}, err
	}
	var (
		ext       []jsonvalue.Member
		condition *jsonvalue.Member
	)
	for _, k := range e.kids {
		switch {
		case isModel(k, "extensionElements"):
			ext, err = decodeMembers(k, map[string]bool{memberID: true, memberFrom: true, memberTo: true, memberCondition: true})
			if err != nil {
				return jsonvalue.Value{}, err
			}
		case isModel(k, "conditionExpression"):
			if len(k.kids) > 0 {
				return jsonvalue.Value{}, formatErrorf("the condition of edge %q has child elements", id)
			}
			condition = &jsonvalue.Member{Key: memberCondition, Value: jsonvalue.String(string(k.text))}
		default:
			return jsonvalue.Value{}, formatErrorf("unexpected element %q in edge %q", k.name.Local, id)
		}
	}
	from := []jsonvalue.Member{{Key: memberNode, Value: jsonvalue.String(source)}}
	members := []jsonvalue.Member{{Key: memberID, Value: jsonvalue.String(id)}}
	var rest []jsonvalue.Member
	for _, m := range ext {
		if m.Key == memberPort {
			from = append(from, m)
			continue
		}
		rest = append(rest, m)
	}
	fromValue, err := jsonvalue.Object(from...)
	if err != nil {
		return jsonvalue.Value{}, formatErrorf("edge %q repeats a member: %v", id, err)
	}
	toValue, err := jsonvalue.Object(jsonvalue.Member{Key: memberNode, Value: jsonvalue.String(target)})
	if err != nil {
		return jsonvalue.Value{}, formatErrorf("edge %q: %v", id, err)
	}
	members = append(members,
		jsonvalue.Member{Key: memberFrom, Value: fromValue},
		jsonvalue.Member{Key: memberTo, Value: toValue})
	if condition != nil {
		members = append(members, *condition)
	}
	members = append(members, rest...)
	v, err := jsonvalue.Object(members...)
	if err != nil {
		return jsonvalue.Value{}, formatErrorf("edge %q repeats a member: %v", id, err)
	}
	return v, nil
}

func strippedRef(e *element, local string) (string, error) {
	ref, ok := e.attr("", local)
	if !ok || !strings.HasPrefix(ref, idNode) {
		return "", formatErrorf("edge has no %s that names a node", local)
	}
	return strings.TrimPrefix(ref, idNode), nil
}

// decodeMembers reads the typed children of an extensionElements element as
// the members of an object. A member named in taken is already set natively
// and may not repeat.
func decodeMembers(ext *element, taken map[string]bool) ([]jsonvalue.Member, error) {
	if !ext.onlySpace() {
		return nil, formatErrorf("text inside extensionElements")
	}
	members := make([]jsonvalue.Member, 0, len(ext.kids))
	for _, k := range ext.kids {
		key, ok := k.attr("", "n")
		if !ok {
			return nil, formatErrorf("an extension member has no name")
		}
		if taken[key] {
			return nil, formatErrorf("the member %q is set twice", key)
		}
		v, err := decodeValue(k, 0)
		if err != nil {
			return nil, err
		}
		members = append(members, jsonvalue.Member{Key: key, Value: v})
	}
	return members, nil
}

// decodeValue reads one typed extension element.
func decodeValue(e *element, depth int) (jsonvalue.Value, error) {
	if e.name.Space != NamespaceOrcon {
		return jsonvalue.Value{}, formatErrorf("unexpected element %q among the extension members", e.name.Local)
	}
	leaf := func() (string, error) {
		if len(e.kids) > 0 {
			return "", formatErrorf("the %s value has child elements", e.name.Local)
		}
		return string(e.text), nil
	}
	switch e.name.Local {
	case kindStr:
		s, err := leaf()
		if err != nil {
			return jsonvalue.Value{}, err
		}
		return jsonvalue.String(s), nil
	case kindNum:
		s, err := leaf()
		if err != nil {
			return jsonvalue.Value{}, err
		}
		v, err := jsonvalue.Number(s)
		if err != nil {
			return jsonvalue.Value{}, formatErrorf("a number value is not a JSON number")
		}
		return v, nil
	case kindBool:
		s, err := leaf()
		if err != nil {
			return jsonvalue.Value{}, err
		}
		switch s {
		case "true":
			return jsonvalue.Bool(true), nil
		case "false":
			return jsonvalue.Bool(false), nil
		default:
			return jsonvalue.Value{}, formatErrorf("a boolean value is neither true nor false")
		}
	case kindNull:
		if s, err := leaf(); err != nil || s != "" {
			return jsonvalue.Value{}, formatErrorf("a null value has content")
		}
		return jsonvalue.Null(), nil
	case kindArr, kindObj:
		return decodeContainer(e, depth)
	default:
		return jsonvalue.Value{}, formatErrorf("unknown value element %q", e.name.Local)
	}
}

func decodeContainer(e *element, depth int) (jsonvalue.Value, error) {
	if depth >= jsonvalue.MaxDepth {
		return jsonvalue.Value{}, formatErrorf("a value nests deeper than %d levels", jsonvalue.MaxDepth)
	}
	if !e.onlySpace() {
		return jsonvalue.Value{}, formatErrorf("text inside a %s value", e.name.Local)
	}
	if e.name.Local == kindArr {
		items := make([]jsonvalue.Value, 0, len(e.kids))
		for _, k := range e.kids {
			if _, named := k.attr("", "n"); named {
				return jsonvalue.Value{}, formatErrorf("an array item has a name")
			}
			v, err := decodeValue(k, depth+1)
			if err != nil {
				return jsonvalue.Value{}, err
			}
			items = append(items, v)
		}
		return jsonvalue.Array(items...), nil
	}
	members := make([]jsonvalue.Member, 0, len(e.kids))
	for _, k := range e.kids {
		key, ok := k.attr("", "n")
		if !ok {
			return jsonvalue.Value{}, formatErrorf("an object member has no name")
		}
		v, err := decodeValue(k, depth+1)
		if err != nil {
			return jsonvalue.Value{}, err
		}
		members = append(members, jsonvalue.Member{Key: key, Value: v})
	}
	v, err := jsonvalue.Object(members...)
	if err != nil {
		return jsonvalue.Value{}, formatErrorf("an object repeats a member: %v", err)
	}
	return v, nil
}
