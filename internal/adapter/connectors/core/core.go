// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package core describes and runs the core data and utility nodes of SPEC-003
// Annex B that the flow engine does not cover itself (cut CN13). No connector
// package owns these types, so the composite catalog asks [Catalog] for them
// (ADR-0029 section 3).
//
// The node types here are pure: they derive their output from their input and
// their parameters only. Nodes that touch the outside world (host access and
// the HTTP node) are not part of this package.
package core

import (
	"context"

	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/flow"
)

// Node types defined by this package. The major version is part of the type,
// so a change of contract is a new type (ADR-0007).
const (
	// TypeNoOp is orcon.noop@1: it passes its input through unchanged.
	TypeNoOp = "orcon.noop@1"
	// TypeSet is orcon.set@1: it assigns fields to every item (n8n Edit Fields).
	TypeSet = "orcon.set@1"
	// TypeStopAndError is orcon.stopAndError@1: it fails the execution with a
	// code the flow author chose (n8n Stop and Error).
	TypeStopAndError = "orcon.stopAndError@1"
)

// types is the static description of every node type of this package. All of
// them are PURE, so they carry no safety contract.
var types = map[string]flow.NodeType{
	TypeNoOp:         {Purity: flow.PurityPure},
	TypeSet:          {Purity: flow.PurityPure},
	TypeStopAndError: {Purity: flow.PurityPure},
}

// Catalog answers for the node types of this package. It is stateless and safe
// for concurrent use. It implements [connector.CoreTypes].
type Catalog struct{}

var _ connector.CoreTypes = (*Catalog)(nil)

// NewCatalog returns the catalog of the core data nodes.
func NewCatalog() *Catalog { return &Catalog{} }

// Lookup implements [flow.NodeTypes]. It reports false for a type this package
// does not define.
func (*Catalog) Lookup(_ context.Context, nodeType string) (flow.NodeType, bool) {
	nt, ok := types[nodeType]
	return nt, ok
}

// ResolveCore implements [connector.CoreTypes]. This package's types have no
// per-node variation, so the node only selects the answer of [Catalog.Lookup].
func (c *Catalog) ResolveCore(ctx context.Context, n *flow.Node) (flow.NodeType, bool) {
	if n == nil {
		return flow.NodeType{}, false
	}
	return c.Lookup(ctx, n.Type)
}
