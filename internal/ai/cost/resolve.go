// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cost

import (
	"crypto/ed25519"
	"errors"
	"time"
)

// Errors of price table verification.
var (
	// ErrSignature reports a signed table that no trusted key verifies.
	ErrSignature = errors.New("cost: price table signature does not verify")
	// ErrExpired reports a signed table past its expiry: it is not trusted.
	ErrExpired = errors.New("cost: price table has expired")
)

// Layer names where a price came from, in the order of resolution (ADR-0062 §3).
const (
	LayerOrgOverride      = "ORG_OVERRIDE"
	LayerInstanceOverride = "INSTANCE_OVERRIDE"
	LayerSigned           = "SIGNED"
	LayerEmbedded         = "EMBEDDED"
)

// VerifySigned checks a price table document against an Ed25519 signature made
// by one of the trusted keys, then reads it. A table that verifies but is past
// its expiry at now is rejected: a stale table is not a price source.
func VerifySigned(doc, sig []byte, keys []ed25519.PublicKey, now time.Time) (Table, error) {
	if !anyVerifies(doc, sig, keys) {
		return Table{}, ErrSignature
	}
	t, err := ParseTable(doc)
	if err != nil {
		return Table{}, err
	}
	if !t.ExpiresAt.IsZero() && !now.Before(t.ExpiresAt) {
		return Table{}, ErrExpired
	}
	return t, nil
}

func anyVerifies(doc, sig []byte, keys []ed25519.PublicKey) bool {
	for _, k := range keys {
		if len(k) == ed25519.PublicKeySize && ed25519.Verify(k, doc, sig) {
			return true
		}
	}
	return false
}

// Resolved is the price chosen for a model and the layer that supplied it.
type Resolved struct {
	Price   Price
	Version string
	Layer   string
}

// Layers holds every source of prices, from the most specific to the most
// general. Resolve applies the order of ADR-0062 §3 and SPEC-006 criterion
// E-COST-1: organization override, then instance override, then the newest
// verified signed table that prices the model, then the table embedded in the
// binary.
type Layers struct {
	// OrgOverride holds prices an organization administrator set.
	OrgOverride map[ModelKey]Price
	// InstanceOverride holds prices the instance administrator set.
	InstanceOverride map[ModelKey]Price
	// Signed holds verified tables from the catalog or an import.
	Signed []Table
	// Embedded is the default table compiled into the binary.
	Embedded Table
}

// Resolve returns the price of a model. The boolean is false when no layer
// prices it: the call then has an unknown cost.
func (l Layers) Resolve(k ModelKey) (Resolved, bool) {
	if p, ok := l.OrgOverride[k]; ok {
		return Resolved{Price: p, Version: "org-override", Layer: LayerOrgOverride}, true
	}
	if p, ok := l.InstanceOverride[k]; ok {
		return Resolved{Price: p, Version: "instance-override", Layer: LayerInstanceOverride}, true
	}
	var best *Table
	for i := range l.Signed {
		t := &l.Signed[i]
		if _, ok := t.Price(k); !ok {
			continue
		}
		if best == nil || t.IssuedAt.After(best.IssuedAt) {
			best = t
		}
	}
	if best != nil {
		p, _ := best.Price(k)
		return Resolved{Price: p, Version: best.Version, Layer: LayerSigned}, true
	}
	if p, ok := l.Embedded.Price(k); ok {
		return Resolved{Price: p, Version: l.Embedded.Version, Layer: LayerEmbedded}, true
	}
	return Resolved{}, false
}
