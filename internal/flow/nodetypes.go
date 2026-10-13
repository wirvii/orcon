// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import "context"

// Purity is the purity class of a node type (ADR-0008 section 5).
type Purity string

// Purity classes.
const (
	// PurityPure derives its output from inputs, parameters and recorded values.
	PurityPure Purity = "PURE"
	// PurityRead reads from outside without side effects.
	PurityRead Purity = "READ"
	// PurityEffect changes the outside world and is journaled per item.
	PurityEffect Purity = "EFFECT"
	// PurityInternalEffect changes Orcon's own state, such as flow data.
	PurityInternalEffect Purity = "INTERNAL_EFFECT"
	// PurityAgentic is a stepped node with its own durable state (ADR-0060).
	PurityAgentic Purity = "AGENTIC"
)

// Purities returns every valid [Purity].
func Purities() []Purity {
	return []Purity{PurityPure, PurityRead, PurityEffect, PurityInternalEffect, PurityAgentic}
}

// NodeType is what validation needs to know about a node type.
type NodeType struct {
	Purity Purity
	// Contract is the safety contract of an EFFECT type; it is required for
	// that purity and ignored for the others.
	Contract *Contract
}

// NodeTypes is the port through which validation learns about node types. The
// connector registry implements it (SPEC-003); tests use a fake catalog.
// Lookup reports false for a type the catalog does not know.
type NodeTypes interface {
	Lookup(ctx context.Context, nodeType string) (NodeType, bool)
}

// isFormatType reports whether the node type is defined by the format itself,
// so validation does not ask the catalog about it.
func isFormatType(nodeType string) bool {
	switch typeName(nodeType) {
	case typeLoop, typeIf, typeSwitch, typeVariables:
		return true
	default:
		return false
	}
}
