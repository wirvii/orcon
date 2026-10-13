// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import (
	"strconv"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// laterElements are the element classes whose native BPMN form belongs to the
// cuts that implement waits, events and call activities (C8, C9).
var laterElements = map[flow.Element]bool{
	flow.ElementBoundaryEvent:          true,
	flow.ElementIntermediateCatchEvent: true,
	flow.ElementIntermediateThrowEvent: true,
	flow.ElementEventBasedGateway:      true,
	flow.ElementCallActivity:           true,
}

// laterBlocks are the node members that need the same later cuts.
var laterBlocks = []string{"multiInstance", "call", "wait", "attachedTo", "cancelActivity", "eventDefinition"}

// supportedElement reports whether the codec has a BPMN form for the class.
func supportedElement(e flow.Element) bool {
	if laterElements[e] {
		return false
	}
	for _, known := range flow.Elements() {
		if known == e {
			return true
		}
	}
	return false
}

// checkScope refuses the nodes of a document value that this version cannot
// carry, naming the first one.
func checkScope(nodes jsonvalue.Value) error {
	for i, n := range nodes.Items() {
		base := pointer("/nodes", strconv.Itoa(i))
		if el, ok := n.Get(memberElement); ok {
			if s, _ := el.Str(); laterElements[flow.Element(s)] {
				return &UnsupportedError{Pointer: pointer(base, memberElement), What: "element " + s}
			}
		}
		for _, block := range laterBlocks {
			if _, ok := n.Get(block); ok {
				return &UnsupportedError{Pointer: pointer(base, block), What: "block " + block}
			}
		}
	}
	return nil
}
