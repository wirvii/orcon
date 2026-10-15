// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cost_test

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/platform/id"
)

var (
	priced   = cost.ModelKey{Provider: "openai", Model: "gpt-x"}
	issuedAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
)

// table builds a table from one priced model.
func table(t *testing.T, version string, issued time.Time, p cost.Price) cost.Table {
	t.Helper()
	tb, err := cost.NewTable(version, issued, map[cost.ModelKey]cost.Price{priced: p})
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

// TestCostFromPriceTable: the cost of a call is the sum of its token kinds at
// the table's prices, each kind rounded up, and the table version is recorded.
func TestCostFromPriceTable(t *testing.T) {
	t.Parallel()
	p := cost.Price{Input: 150_000, Output: 600_000, CacheRead: 75_000}
	tb := table(t, "2026-10-01", issuedAt, p)

	got, ok := tb.Price(priced)
	if !ok {
		t.Fatal("the priced model is missing from its own table")
	}
	usage := cost.Usage{Input: 1000, Output: 500, CacheRead: 2000}
	// Input 1000 tokens at 150000 gives 150; output 500 at 600000 gives 300; cache reads 2000 at 75000 give 150.
	if c := cost.UsageCost(got, usage); c != 600 {
		t.Errorf("UsageCost() = %d micro-dollars, want 600", c)
	}
	if c := cost.UsageCost(got, cost.Usage{Input: 1}); c != 1 {
		t.Errorf("one input token costs %d, want it rounded up to 1", c)
	}
	if r := cost.Reservation(got, 1000, 100); r != 150+60 {
		t.Errorf("Reservation() = %d, want 210", r)
	}
	if tb.Version != "2026-10-01" {
		t.Errorf("version = %q, want the table version recorded with the cost", tb.Version)
	}
}

// TestPriceResolutionOrder: organization override > instance override > the
// newest verified signed table that prices the model > embedded table.
func TestPriceResolutionOrder(t *testing.T) {
	t.Parallel()
	embedded := table(t, "embedded", issuedAt, cost.Price{Input: 1})
	older := table(t, "signed-old", issuedAt.AddDate(0, -1, 0), cost.Price{Input: 2})
	newer := table(t, "signed-new", issuedAt, cost.Price{Input: 3})
	org := map[cost.ModelKey]cost.Price{priced: {Input: 4}}
	instance := map[cost.ModelKey]cost.Price{priced: {Input: 5}}

	cases := []struct {
		name   string
		layers cost.Layers
		want   string
		price  int64
	}{
		{"org override wins over everything", cost.Layers{OrgOverride: org, InstanceOverride: instance, Signed: []cost.Table{newer}, Embedded: embedded}, cost.LayerOrgOverride, 4},
		{"instance override wins over signed", cost.Layers{InstanceOverride: instance, Signed: []cost.Table{newer}, Embedded: embedded}, cost.LayerInstanceOverride, 5},
		{"newest signed table wins", cost.Layers{Signed: []cost.Table{older, newer}, Embedded: embedded}, cost.LayerSigned, 3},
		{"embedded is the last resort", cost.Layers{Embedded: embedded}, cost.LayerEmbedded, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, ok := c.layers.Resolve(priced)
			if !ok || got.Layer != c.want || got.Price.Input != c.price {
				t.Errorf("Resolve() = %+v, %v; want layer %s price %d", got, ok, c.want, c.price)
			}
		})
	}

	if _, ok := (cost.Layers{Embedded: cost.Embedded()}).Resolve(priced); ok {
		t.Error("the shipped embedded table priced a model; it must ship without prices")
	}
}

// TestUnknownPriceBlock: a model without a price has an unknown cost; a budget
// that blocks unpriced calls refuses it, and a budget that allows them does not.
func TestUnknownPriceBlock(t *testing.T) {
	t.Parallel()
	scope := cost.Scope{Kind: cost.ScopeOrg, ID: id.Derive("org")}
	blocking := cost.BudgetState{Budget: cost.Budget{Scope: scope, Period: cost.PeriodMonth, LimitMicro: 1000, UnknownPrice: cost.UnknownPriceBlock}}
	allowing := cost.BudgetState{Budget: cost.Budget{Scope: scope, Period: cost.PeriodMonth, LimitMicro: 1000, UnknownPrice: cost.UnknownPriceAllow}}

	if _, err := cost.Decide([]cost.BudgetState{blocking}, 0, false); !errors.Is(err, cost.ErrUnknownPrice) {
		t.Errorf("blocking budget with an unpriced model = %v, want ErrUnknownPrice", err)
	}
	if _, err := cost.Decide([]cost.BudgetState{allowing}, 0, false); err != nil {
		t.Errorf("allowing budget with an unpriced model = %v, want the call to run", err)
	}
	if _, err := cost.Decide([]cost.BudgetState{blocking}, 0, true); err != nil {
		t.Errorf("blocking budget with a priced model = %v, want the call to run", err)
	}
}

// publicOf returns the public half of a private key, failing the test otherwise.
func publicOf(t *testing.T, priv ed25519.PrivateKey) ed25519.PublicKey {
	t.Helper()
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		t.Fatal("an Ed25519 key has no Ed25519 public key")
	}
	return pub
}

// TestSignedPriceTableVerification: a table must verify against a trusted key,
// must parse, and must not be past its expiry.
func TestSignedPriceTableVerification(t *testing.T) {
	t.Parallel()
	priv := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	pub := publicOf(t, priv)
	other := publicOf(t, ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize)))
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)

	doc := []byte(fmt.Sprintf(`{"format":"orcon/ai-prices@1","version":"v1","issuedAt":%q,"expiresAt":%q,"models":[{"provider":"openai","model":"gpt-x","inputPerMillion":150000}]}`,
		now.Add(-time.Hour).Format(time.RFC3339), now.AddDate(0, 1, 0).Format(time.RFC3339)))
	sig := ed25519.Sign(priv, doc)

	t.Run("a valid signature and a live expiry are accepted", func(t *testing.T) {
		t.Parallel()
		tb, err := cost.VerifySigned(doc, sig, []ed25519.PublicKey{other, pub}, now)
		if err != nil {
			t.Fatalf("VerifySigned() error = %v", err)
		}
		if p, ok := tb.Price(priced); !ok || p.Input != 150000 {
			t.Errorf("price = %+v, %v", p, ok)
		}
	})

	t.Run("a tampered document is rejected", func(t *testing.T) {
		t.Parallel()
		tampered := bytes.Replace(doc, []byte("150000"), []byte("1"), 1)
		if _, err := cost.VerifySigned(tampered, sig, []ed25519.PublicKey{pub}, now); !errors.Is(err, cost.ErrSignature) {
			t.Errorf("tampered table = %v, want ErrSignature", err)
		}
	})

	t.Run("a table signed by an untrusted key is rejected", func(t *testing.T) {
		t.Parallel()
		if _, err := cost.VerifySigned(doc, sig, []ed25519.PublicKey{other}, now); !errors.Is(err, cost.ErrSignature) {
			t.Errorf("untrusted key = %v, want ErrSignature", err)
		}
	})

	t.Run("an expired table is rejected", func(t *testing.T) {
		t.Parallel()
		later := now.AddDate(1, 0, 0)
		if _, err := cost.VerifySigned(doc, sig, []ed25519.PublicKey{pub}, later); !errors.Is(err, cost.ErrExpired) {
			t.Errorf("expired table = %v, want ErrExpired", err)
		}
	})

	t.Run("a table in the wrong format is rejected", func(t *testing.T) {
		t.Parallel()
		bad := []byte(strings.Replace(string(doc), "orcon/ai-prices@1", "other@1", 1))
		if _, err := cost.ParseTable(bad); err == nil {
			t.Error("ParseTable accepted a foreign format")
		}
	})
}
