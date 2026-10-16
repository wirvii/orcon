// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cost

import (
	"errors"
	"fmt"
)

// Purpose says why a model call was made. Every cost record carries one, so
// budgets can report spend by use (SPEC-006 R-COST-07).
type Purpose string

// The purposes of a model call.
const (
	PurposeAgent      Purpose = "AGENT"
	PurposeChain      Purpose = "CHAIN"
	PurposeEmbedding  Purpose = "EMBEDDING"
	PurposeEvaluation Purpose = "EVALUATION"
	PurposeMemory     Purpose = "MEMORY"
	PurposeRetrieval  Purpose = "RETRIEVAL"
)

// Valid reports whether p is one of the closed purposes.
func (p Purpose) Valid() bool {
	switch p {
	case PurposeAgent, PurposeChain, PurposeEmbedding, PurposeEvaluation, PurposeMemory, PurposeRetrieval:
		return true
	}
	return false
}

// Usage counts the tokens of one call, by kind. Input excludes cache reads and
// writes, which have their own counts (provider.Usage has the same meaning).
type Usage struct {
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
	Reasoning  int64
}

// perMillion is the unit a [Price] is written in.
const perMillion = 1_000_000

// UsageCost returns the estimated cost of a call in micro-dollars. Each kind of
// token is rounded up on its own, so a non-empty count never costs nothing.
func UsageCost(p Price, u Usage) int64 {
	return ceilMicros(u.Input, p.Input) +
		ceilMicros(u.Output, p.Output) +
		ceilMicros(u.CacheRead, p.CacheRead) +
		ceilMicros(u.CacheWrite, p.CacheWrite) +
		ceilMicros(u.Reasoning, p.Reasoning)
}

// Reservation is the most a call may cost before it is made: the counted input
// at the input price plus the largest allowed output at the output price
// (ADR-0062 §4). The budget check uses it, so a call that would overshoot is
// refused before it is sent.
func Reservation(p Price, inputTokens, maxOutputTokens int64) int64 {
	return ceilMicros(inputTokens, p.Input) + ceilMicros(maxOutputTokens, p.Output)
}

func ceilMicros(tokens, pricePerMillion int64) int64 {
	if tokens <= 0 || pricePerMillion <= 0 {
		return 0
	}
	return (tokens*pricePerMillion + perMillion - 1) / perMillion
}

// Entry is one priced model call, ready to be recorded.
type Entry struct {
	Purpose      Purpose
	Provider     string
	Model        string
	Usage        Usage
	CostMicro    int64
	Priced       bool
	PriceVersion string
}

// Validate checks the entry before it is stored.
func (e Entry) Validate() error {
	if !e.Purpose.Valid() {
		return fmt.Errorf("cost: purpose %q is not one of the closed set", e.Purpose)
	}
	if e.Provider == "" || e.Model == "" {
		return errors.New("cost: an entry needs a provider and a model")
	}
	if e.CostMicro < 0 {
		return errors.New("cost: an entry cannot cost less than zero")
	}
	return nil
}
