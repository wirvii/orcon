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
			nt, known = v.cfg.lookup(n.Type)
			if !known {
				v.r.errorf(CodeV10, base+"/type", "node %q has type %q, which the node-type catalog does not know", n.ID, n.Type)
				continue
			}
			v.contract(base+"/type", n, nt)
		}
		if n.Attention != nil {
			v.attention(base+"/attention", n, nt, known)
		}
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
