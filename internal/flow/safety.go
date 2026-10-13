// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

// safety runs the effect rules of ADR-0007 section 9 level 5 and
// ADR-0010 section 11 rule V10, plus the attention override of ADR-0073.
func (v *validator) safety() {
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		base := ptr("nodes", i)
		if n.Effect != nil {
			v.contract(base+"/effect", n)
		}
		if n.Attention != nil {
			v.attention(base+"/attention", n)
		}
	}
}

// contract implements V10 for one node: a write, delete or unknown effect
// must say how an ambiguous outcome is recovered, and the recovery it names
// must be backed by the rest of the contract (ADR-0012 sections 1 and 6).
func (v *validator) contract(path string, n *Node) {
	e := n.Effect
	if e.Kind == EffectRead {
		return
	}
	if e.OnUncertain == "" {
		v.r.errorf(CodeV10, path+"/onUncertain", "node %q has a %s effect, so its contract must say onUncertain", n.ID, e.Kind)
		return
	}
	switch e.OnUncertain {
	case OnUncertainResendWithKey:
		v.resendNeedsKey(path, n)
	case OnUncertainReconcile:
		if e.Verification == nil || !e.Verification.Reconcile {
			v.r.errorf(CodeV10, path+"/verification", "node %q recovers with RECONCILE, so verification.reconcile must be true", n.ID)
		}
	case OnUncertainSuspend:
		// Always valid: a person decides.
	}
}

func (v *validator) resendNeedsKey(path string, n *Node) {
	idem := n.Effect.Idempotency
	switch {
	case idem == nil || idem.Mode == IdempotencyNone:
		v.r.errorf(CodeV10, path+"/idempotency", "node %q recovers with RESEND_WITH_KEY, so it needs an idempotency mode of PROVIDER_KEY or NATURAL_KEY", n.ID)
	case idem.TTL == "":
		v.r.errorf(CodeV10, path+"/idempotency/ttl", "node %q recovers with RESEND_WITH_KEY, so the idempotency key needs a ttl (a key older than it is never resent)", n.ID)
	}
}

// attention validates the notification override. OFF is an error on a node
// whose uncertain operations always ask a person, and the block makes no sense
// on a node that neither waits nor has an effect.
func (v *validator) attention(path string, n *Node) {
	hasEffect := n.Effect != nil
	waits := n.Wait != nil || n.Element == ElementUserTask
	if !hasEffect && !waits {
		v.r.errorf(CodeAttention, path, "node %q has an attention block but neither waits for a person nor has an effect", n.ID)
		return
	}
	if n.Attention.Notify == AttentionOff && hasEffect && n.Effect.Kind != EffectRead {
		v.r.errorf(CodeAttention, path+"/notify", "node %q has a write effect; notifying a person about an uncertain operation is mandatory and cannot be turned OFF", n.ID)
	}
}
