// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package cost is the cost accounting of model calls (SPEC-006 cut AI2, ADR-0062):
// the price table and how prices are resolved, the cost of one call, the
// budgets that stop or suspend a run, and the usage rollups the budgets read.
//
// The package is an area package. Its persistence is the [Store] port, which the
// SQLite and PostgreSQL adapters implement; it imports no adapter.
package cost

import (
	"errors"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// TableFormat is the format tag of a price table document (ADR-0062 §3).
const TableFormat = "orcon/ai-prices@1"

// MaxPricePerMillion bounds one price so a cost computation cannot overflow
// int64 for any realistic token count. It is micro-dollars per million tokens.
const MaxPricePerMillion = 1_000_000_000

// ModelKey names a model in a price table.
type ModelKey struct {
	Provider string
	Model    string
}

// Price is what one million tokens of each kind cost, in micro-dollars
// (1 USD = 1,000,000). Reasoning is priced separately only when the provider
// bills reasoning tokens apart from output; otherwise it stays zero.
type Price struct {
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
	Reasoning  int64
}

// Table is one price table: a version, where it came from, and its prices.
type Table struct {
	// Version identifies the table in every cost record (priceTableVersion).
	Version string
	// IssuedAt orders signed tables: the newest issue wins for a model.
	IssuedAt time.Time
	// ExpiresAt is when the table stops being trusted. Zero means it never expires.
	ExpiresAt time.Time
	prices    map[ModelKey]Price
}

// NewTable builds a table from prices. It validates every price.
func NewTable(version string, issued time.Time, prices map[ModelKey]Price) (Table, error) {
	if version == "" {
		return Table{}, errors.New("cost: a price table needs a version")
	}
	cp := make(map[ModelKey]Price, len(prices))
	for k, p := range prices {
		if err := p.validate(); err != nil {
			return Table{}, fmt.Errorf("cost: price of %s/%s: %w", k.Provider, k.Model, err)
		}
		cp[k] = p
	}
	return Table{Version: version, IssuedAt: issued, prices: cp}, nil
}

// Price returns the price of a model in this table.
func (t Table) Price(k ModelKey) (Price, bool) {
	p, ok := t.prices[k]
	return p, ok
}

// Len returns the number of priced models.
func (t Table) Len() int { return len(t.prices) }

func (p Price) validate() error {
	for name, v := range map[string]int64{
		"input": p.Input, "output": p.Output, "cacheRead": p.CacheRead, "cacheWrite": p.CacheWrite, "reasoning": p.Reasoning,
	} {
		if v < 0 || v > MaxPricePerMillion {
			return fmt.Errorf("%s price %d is outside 0..%d", name, v, MaxPricePerMillion)
		}
	}
	return nil
}

// ParseTable reads a price table document in the orcon/ai-prices@1 format:
//
//	{"format":"orcon/ai-prices@1","version":"...","issuedAt":"RFC 3339",
//	 "expiresAt":"RFC 3339 (optional)","models":[{"provider":"openai","model":"gpt-x",
//	 "inputPerMillion":0,"outputPerMillion":0,"cacheReadPerMillion":0,
//	 "cacheWritePerMillion":0,"reasoningPerMillion":0}]}
//
// Prices are whole micro-dollars per million tokens; a document that is not in
// this shape is rejected.
func ParseTable(doc []byte) (Table, error) {
	v, err := jsonvalue.Parse(doc)
	if err != nil {
		return Table{}, fmt.Errorf("cost: price table is not JSON: %w", err)
	}
	if s := stringField(v, "format"); s != TableFormat {
		return Table{}, fmt.Errorf("cost: price table format %q, want %s", s, TableFormat)
	}
	issued, err := timeField(v, "issuedAt", true)
	if err != nil {
		return Table{}, err
	}
	expires, err := timeField(v, "expiresAt", false)
	if err != nil {
		return Table{}, err
	}
	models, ok := v.Get("models")
	if !ok || models.Kind() != jsonvalue.KindArray {
		return Table{}, errors.New("cost: price table needs a models array")
	}
	prices := make(map[ModelKey]Price, models.Len())
	for i, m := range models.Items() {
		key := ModelKey{Provider: stringField(m, "provider"), Model: stringField(m, "model")}
		if key.Provider == "" || key.Model == "" {
			return Table{}, fmt.Errorf("cost: models[%d] needs a provider and a model", i)
		}
		p, err := priceField(m)
		if err != nil {
			return Table{}, fmt.Errorf("cost: models[%d]: %w", i, err)
		}
		if _, dup := prices[key]; dup {
			return Table{}, fmt.Errorf("cost: %s/%s is listed twice", key.Provider, key.Model)
		}
		prices[key] = p
	}
	t, err := NewTable(stringField(v, "version"), issued, prices)
	if err != nil {
		return Table{}, err
	}
	t.ExpiresAt = expires
	return t, nil
}

func priceField(m jsonvalue.Value) (Price, error) {
	var p Price
	fields := []struct {
		key string
		dst *int64
	}{
		{"inputPerMillion", &p.Input},
		{"outputPerMillion", &p.Output},
		{"cacheReadPerMillion", &p.CacheRead},
		{"cacheWritePerMillion", &p.CacheWrite},
		{"reasoningPerMillion", &p.Reasoning},
	}
	for _, f := range fields {
		v, ok := m.Get(f.key)
		if !ok {
			continue
		}
		n, ok := v.Int64()
		if !ok {
			return Price{}, fmt.Errorf("%s must be a whole number of micro-dollars", f.key)
		}
		*f.dst = n
	}
	if err := p.validate(); err != nil {
		return Price{}, err
	}
	return p, nil
}

func stringField(v jsonvalue.Value, key string) string {
	f, ok := v.Get(key)
	if !ok {
		return ""
	}
	s, _ := f.Str()
	return s
}

func timeField(v jsonvalue.Value, key string, required bool) (time.Time, error) {
	s := stringField(v, key)
	if s == "" {
		if required {
			return time.Time{}, fmt.Errorf("cost: price table needs %s", key)
		}
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("cost: %s is not an RFC 3339 instant: %w", key, err)
	}
	return t.UTC(), nil
}
