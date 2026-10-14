// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import (
	"context"
	"errors"
	"fmt"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// Catalog is the node-type catalog of one project. It implements
// [flow.ConnectorNodeTypes], so validation level 5 can ask it about the
// connector nodes of a flow (ADR-0007 section 9, ADR-0034 section 1).
type Catalog struct {
	registry *Registry
	org      identity.OrgID
	project  identity.ProjectID
}

var _ flow.ConnectorNodeTypes = (*Catalog)(nil)

// Lookup implements [flow.NodeTypes]. It describes the node type as its newest
// installed version defines it, which is what a node without a pin would use.
// A lookup that fails to reach the store reports the type as unknown.
func (c *Catalog) Lookup(ctx context.Context, nodeType string) (flow.NodeType, bool) {
	connectorID, ok := flow.ConnectorIDOf(nodeType)
	if !ok {
		return flow.NodeType{}, false
	}
	_, m, found, err := c.registry.latest(ctx, connectorID, nodeType)
	if err != nil || !found {
		return flow.NodeType{}, false
	}
	n, _ := m.NodeByType(nodeType)
	return nodeTypeOf(n), true
}

// ResolveNode implements [flow.ConnectorNodeTypes]. A node pins the exact
// package version it uses, so the answer comes from that version's manifest.
func (c *Catalog) ResolveNode(ctx context.Context, nodeType string, pin *flow.ConnectorPin, operation string) flow.NodeResolution {
	connectorID, owned := flow.ConnectorIDOf(nodeType)
	if !owned {
		return flow.NodeResolution{}
	}
	if pin == nil {
		return c.resolveWithoutPin(ctx, connectorID, nodeType, operation)
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
	n, has := m.NodeByType(nodeType)
	if !has {
		return flow.NodeResolution{Problems: []string{fmt.Sprintf("%s version %s has no node type %s", connectorID, pin.Version, nodeType)}}
	}
	res := flow.NodeResolution{Known: true}
	res.Problems = c.pinProblems(ctx, &rec, &m, pin)
	res.Type, res.UnknownOperation = resolveOperation(n, operation)
	return res
}

func (c *Catalog) resolveWithoutPin(ctx context.Context, connectorID, nodeType, operation string) flow.NodeResolution {
	rec, m, found, err := c.registry.latest(ctx, connectorID, nodeType)
	if err != nil {
		return flow.NodeResolution{Problems: []string{fmt.Sprintf("could not read the installed %s versions: %v", connectorID, err)}}
	}
	if !found {
		return flow.NodeResolution{}
	}
	n, _ := m.NodeByType(nodeType)
	res := flow.NodeResolution{Known: true, Problems: []string{
		fmt.Sprintf("has no connector pin; nodes of %s record the version and digest they were built against (latest installed is %s)", connectorID, rec.Version),
	}}
	res.Type, res.UnknownOperation = resolveOperation(n, operation)
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

// resolveOperation describes a node as validation needs it. A node that selects
// an operation gets that operation's purity and contract. A node that selects
// none gets the most restrictive of its operations, so no flow looks safer than
// the riskiest thing the node can do.
func resolveOperation(n *Node, operation string) (flow.NodeType, bool) {
	if operation != "" {
		op, ok := n.OperationByName(operation)
		if !ok {
			return flow.NodeType{}, true
		}
		return nodeTypeOfOperation(n, op), false
	}
	return nodeTypeOf(n), false
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
