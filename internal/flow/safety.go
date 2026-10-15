// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import "strings"

// safety runs the node-type rules of ADR-0007 section 9 level 5, rule V10, and
// the attention override of ADR-0073. Without a node-type catalog it has
// nothing to check against and does nothing.
func (v *validator) safety() {
	if v.cfg.lookup == nil {
		return
	}
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		base := ptr("nodes", i)
		var nt NodeType
		known := false
		if !isFormatType(n.Type) {
			var ok bool
			if nt, ok = v.resolve(base, n); !ok {
				continue
			}
			known = true
			v.contract(base+"/type", n, nt)
		}
		if n.Attention != nil {
			v.attention(base+"/attention", n, nt, known)
		}
	}
}

// resolve finds the node type of a node. With a catalog that knows connector
// packages it asks for the pinned version and the selected operation and
// reports the reasons the pin or the selection cannot be used; otherwise it is
// the plain lookup. A type no package can own is never judged on selection,
// and when the resolver does not know it the plain lookup gets a second
// chance before the type is declared unknown. It reports false when the type
// cannot be used, after recording why.
func (v *validator) resolve(base string, n *Node) (NodeType, bool) {
	unknown := func() {
		v.r.errorf(CodeV10, base+"/type", "node %q has type %q, which the node-type catalog does not know", n.ID, n.Type)
	}
	if v.cfg.resolve == nil {
		nt, known := v.cfg.lookup(n.Type)
		if !known {
			unknown()
		}
		return nt, known
	}
	res := v.cfg.resolve(n)
	_, packaged := ConnectorIDOf(n.Type)
	if !res.Known && len(res.Problems) == 0 && !packaged {
		if nt, ok := v.cfg.lookup(n.Type); ok {
			res = NodeResolution{Type: nt, Known: true}
		}
	}
	if !res.Known && len(res.Problems) == 0 {
		unknown()
		return NodeType{}, false
	}
	usable := res.Known
	if packaged && res.Known && res.Selection != SelectionOK {
		v.selection(base+"/parameters/operation", n, res)
		usable = false
	}
	for _, problem := range res.Problems {
		v.r.errorf(CodeConnector, base+"/connector", "node %q: %s", n.ID, strings.TrimSpace(problem))
	}
	return res.Type, usable
}

// selection reports a selection problem of a package node (rule V10).
func (v *validator) selection(path string, n *Node, res NodeResolution) {
	switch res.Selection {
	case SelectionOK:
		return
	case SelectionMissing:
		v.r.errorf(CodeV10, path, "node %q must select the operation it runs with the literal parameter \"operation\" (resource.verb), even when type %q has a single operation; it has: %s", n.ID, n.Type, strings.Join(res.Operations, ", "))
	case SelectionNotLiteral:
		v.r.errorf(CodeV10, path, "node %q selects its operation with an expression; the \"operation\" parameter must be a literal resource.verb so publication can read the safety contract of what it runs", n.ID)
	case SelectionUnknown:
		v.r.errorf(CodeV10, path, "node %q selects operation %q, which type %q does not have in the pinned version; it has: %s", n.ID, SelectionOf(n).Operation, n.Type, strings.Join(res.Operations, ", "))
	case SelectionOnTrigger:
		v.r.errorf(CodeV10, path, "node %q is a trigger; trigger node types select no operation, remove the \"operation\" parameter", n.ID)
	}
}

// contract implements V10 for one node: an effect type must carry a valid
// safety contract (ADR-0029 section 5; the rules are [CheckContract]).
func (v *validator) contract(path string, n *Node, nt NodeType) {
	if nt.Purity != PurityEffect {
		return
	}
	if nt.Contract == nil {
		v.r.errorf(CodeV10, path, "node %q has effect type %q without a safety contract", n.ID, n.Type)
		return
	}
	for _, problem := range CheckContract(nt.Contract) {
		v.r.errorf(CodeV10, path, "the contract of %q is invalid: %s", n.Type, strings.TrimSpace(problem))
	}
}

// attention validates the notification override. OFF is an error on a node
// whose uncertain operations always ask a person, and the block makes no sense
// on a node that neither waits nor is an effect node.
func (v *validator) attention(path string, n *Node, nt NodeType, known bool) {
	isEffect := known && nt.Purity == PurityEffect
	waits := n.Wait != nil || n.Element == ElementUserTask
	if !isEffect && !waits {
		v.r.errorf(CodeAttention, path, "node %q has an attention block but neither waits for a person nor is an effect node", n.ID)
		return
	}
	writes := isEffect && nt.Contract != nil && nt.Contract.Effect != EffectRead
	if n.Attention.Notify == AttentionOff && writes {
		v.r.errorf(CodeAttention, path+"/notify", "node %q has a write effect; notifying a person about an uncertain operation is mandatory and cannot be turned OFF", n.ID)
	}
}
