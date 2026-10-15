// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import (
	"context"
	"errors"
	"fmt"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// CoreTypes describes node types that no connector package owns: orcon.http@1
// and the Annex B nodes of SPEC-003. It is implemented outside this package
// and wired by the composition root; a [Catalog] built without one knows none.
type CoreTypes interface {
	// Lookup is the type-level answer used by palettes and views: the worst
	// case over the node's parameters.
	flow.NodeTypes
	// ResolveCore is the node-aware answer publication uses; the same rule
	// must drive dispatch. It reports false for a type it does not define.
	ResolveCore(ctx context.Context, n *flow.Node) (flow.NodeType, bool)
}

// Catalog is the node-type catalog of one project. It is the composite catalog
// of level 5: connector packages answer for package node types and the
// [CoreTypes] for every other type. It implements [flow.ConnectorNodeTypes]
// (ADR-0007 section 9, ADR-0034 section 1).
type Catalog struct {
	registry *Registry
	org      orgscope.Org
	project  identity.ProjectID
	core     CoreTypes
}

var _ flow.ConnectorNodeTypes = (*Catalog)(nil)

// Lookup implements [flow.NodeTypes]. A package type is described as its
// newest installed version defines it, with its riskiest operation: this is
// informative, publication uses [Catalog.ResolveNode]. Any other type is asked
// of the core types. A lookup that fails to reach the store reports the type
// as unknown.
func (c *Catalog) Lookup(ctx context.Context, nodeType string) (flow.NodeType, bool) {
	connectorID, ok := flow.ConnectorIDOf(nodeType)
	if !ok {
		if c.core == nil {
			return flow.NodeType{}, false
		}
		return c.core.Lookup(ctx, nodeType)
	}
	_, m, found, err := c.registry.latest(ctx, connectorID, nodeType)
	if err != nil || !found {
		return flow.NodeType{}, false
	}
	n, _ := m.NodeByType(nodeType)
	return nodeTypeOf(n), true
}

// ResolveNode implements [flow.ConnectorNodeTypes]. A node of a type no package
// owns is answered by the core types and never meets pin or selection rules.
// A package node pins the exact package version it uses, so the answer comes
// from that version's manifest, with the contract of the operation it selects.
func (c *Catalog) ResolveNode(ctx context.Context, n *flow.Node) flow.NodeResolution {
	connectorID, owned := flow.ConnectorIDOf(n.Type)
	if !owned {
		if c.core == nil {
			return flow.NodeResolution{}
		}
		nt, known := c.core.ResolveCore(ctx, n)
		return flow.NodeResolution{Type: nt, Known: known}
	}
	pin := n.Connector
	if pin == nil {
		return c.resolveWithoutPin(ctx, connectorID, n)
	}
	rec, err := c.registry.packages.GetPackage(ctx, connectorID, pin.Version)
	if errors.Is(err, ErrNotFound) {
		return flow.NodeResolution{Problems: []string{fmt.Sprintf("pins %s version %s, which is not installed", connectorID, pin.Version)}}
	}
	if err != nil {
		return flow.NodeResolution{Problems: []string{fmt.Sprintf("could not read the installed %s versions: %v", connectorID, err)}}
	}
	m, err := Parse(rec.Manifest, Options{Official: true})
	if err != nil {
		return flow.NodeResolution{Problems: []string{fmt.Sprintf("the installed manifest of %s %s is unreadable: %v", connectorID, pin.Version, err)}}
	}
	node, has := m.NodeByType(n.Type)
	if !has {
		return flow.NodeResolution{Problems: []string{fmt.Sprintf("%s version %s has no node type %s", connectorID, pin.Version, n.Type)}}
	}
	res := resolveSelection(node, flow.SelectionOf(n))
	res.Problems = c.pinProblems(ctx, &rec, &m, pin)
	return res
}

func (c *Catalog) resolveWithoutPin(ctx context.Context, connectorID string, n *flow.Node) flow.NodeResolution {
	rec, m, found, err := c.registry.latest(ctx, connectorID, n.Type)
	if err != nil {
		return flow.NodeResolution{Problems: []string{fmt.Sprintf("could not read the installed %s versions: %v", connectorID, err)}}
	}
	if !found {
		return flow.NodeResolution{}
	}
	node, _ := m.NodeByType(n.Type)
	res := resolveSelection(node, flow.SelectionOf(n))
	res.Problems = []string{
		fmt.Sprintf("has no connector pin; nodes of %s record the version and digest they were built against (latest installed is %s)", connectorID, rec.Version),
	}
	return res
}

// pinProblems checks the digest, the grant and the policy of a pinned version.
func (c *Catalog) pinProblems(ctx context.Context, rec *PackageRecord, m *Manifest, pin *flow.ConnectorPin) []string {
	var problems []string
	if rec.Status != StatusInstalled {
		problems = append(problems, fmt.Sprintf("pins %s version %s, which is retired", rec.ConnectorID, rec.Version))
	}
	if rec.Digest != pin.Digest {
		problems = append(problems, fmt.Sprintf("pins %s version %s with digest %s, but the installed package has digest %s", rec.ConnectorID, rec.Version, pin.Digest, rec.Digest))
	}
	if _, err := c.registry.grants.FindGrant(ctx, c.org, rec.ID, c.project); err != nil {
		if errors.Is(err, ErrNotFound) {
			problems = append(problems, fmt.Sprintf("uses %s version %s, which the organization has not granted", rec.ConnectorID, rec.Version))
		} else {
			problems = append(problems, fmt.Sprintf("could not check whether %s version %s is granted: %v", rec.ConnectorID, rec.Version, err))
		}
	}
	if err := c.registry.policy.Check(ctx, m); err != nil {
		problems = append(problems, fmt.Sprintf("uses %s version %s, which the connector policy blocks: %v", rec.ConnectorID, rec.Version, err))
	}
	return problems
}

// resolveSelection describes a package node as publication needs it. An action
// node runs exactly the operation it selects: its purity and contract are that
// operation's, and a missing, non-literal or unknown selection is reported
// instead of guessing the riskiest operation (SPEC-003 rules S-1 to S-4).
// A trigger node selects nothing and must not carry a selector.
func resolveSelection(n *Node, sel flow.Selection) flow.NodeResolution {
	res := flow.NodeResolution{Known: true}
	if n.Group == GroupTrigger {
		res.Type = nodeTypeOf(n)
		if sel.Present {
			res.Selection = flow.SelectionOnTrigger
		}
		return res
	}
	res.Operations = make([]string, len(n.Operations))
	for i := range n.Operations {
		res.Operations[i] = n.Operations[i].Name
	}
	switch {
	case !sel.Present:
		res.Selection = flow.SelectionMissing
	case !sel.Literal:
		res.Selection = flow.SelectionNotLiteral
	default:
		op, ok := n.OperationByName(sel.Operation)
		if !ok {
			res.Selection = flow.SelectionUnknown
			return res
		}
		res.Type = nodeTypeOfOperation(n, op)
	}
	return res
}

func nodeTypeOf(n *Node) flow.NodeType {
	if len(n.Operations) == 0 {
		// A trigger reads from outside and has no safety contract.
		return flow.NodeType{Purity: flow.PurityRead}
	}
	worst := &n.Operations[0]
	for i := 1; i < len(n.Operations); i++ {
		if riskier(&n.Operations[i], worst) {
			worst = &n.Operations[i]
		}
	}
	return nodeTypeOfOperation(n, worst)
}

func nodeTypeOfOperation(n *Node, op *Operation) flow.NodeType {
	return flow.NodeType{Purity: op.Purity, Contract: n.Contract(op)}
}

// purityRisk orders purities from the least to the most consequential.
var purityRisk = map[flow.Purity]int{
	flow.PurityPure: 0, flow.PurityRead: 1, flow.PurityInternalEffect: 2, flow.PurityEffect: 3,
}

// levelRisk orders safety levels from the safest to the weakest.
var levelRisk = map[flow.SafetyLevel]int{
	flow.SafetyReadOnly: 0, flow.SafetyIdempotencyKey: 1, flow.SafetyReconcile: 2, flow.SafetyUnsafe: 3,
}

// effectRisk orders effects from the harmless to the most destructive.
var effectRisk = map[flow.EffectKind]int{
	flow.EffectRead: 0, flow.EffectWrite: 1, flow.EffectDelete: 2, flow.EffectUnknown: 3,
}

// riskier reports whether operation a is riskier than b: a higher purity, then
// a weaker safety level, then a more destructive effect.
func riskier(a, b *Operation) bool {
	if purityRisk[a.Purity] != purityRisk[b.Purity] {
		return purityRisk[a.Purity] > purityRisk[b.Purity]
	}
	if a.Safety == nil || b.Safety == nil {
		return a.Safety != nil && b.Safety == nil
	}
	if levelRisk[a.Safety.Level] != levelRisk[b.Safety.Level] {
		return levelRisk[a.Safety.Level] > levelRisk[b.Safety.Level]
	}
	return effectRisk[a.Safety.Effect] > effectRisk[b.Safety.Effect]
}
