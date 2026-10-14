// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package examplestest runs the executable examples of the public API contract
// against a live server (orcon-api, SPEC-012 section 4.6).
//
// Every operation of the bundled contract (packages/openapi/v1/openapi.bundle.json)
// lists its cases under x-orcon-examples. For each case the runner:
//
//  1. runs the named scenario, a Go function that seeds state through the public
//     API only and returns Bindings from the fixed ids and ETags that appear in
//     the examples to the real ones;
//  2. substitutes the bindings, sends the request and requires the status;
//  3. validates the body against the response schema (JSON Schema 2020-12) and
//     compares every leaf of the example with the real body: equal after
//     bindings, except uuid and date-time values the scenario did not bind and
//     fields marked x-orcon-volatile, which are compared by format or presence;
//  4. polls Location until the operation is terminal for long-running cases and
//     watches the event stream for stream cases.
//
// The package is test support. It is imported by tests only; the product binary
// never links it. TestRunnerAgainstFake proves the runner against a fake server
// and a contract with planted faults, with no binary needed.
package examplestest

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
)

// Bindings maps a fixed value used in the examples (an id, an ETag) to the real
// value a scenario created.
type Bindings map[string]string

// Scenario seeds the state one example needs. It uses api, the public API, and
// nothing else: never storage, never the internet. It must pass on SQLite and
// PostgreSQL.
type Scenario func(ctx context.Context, api *Client) (Bindings, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Scenario{}
)

// RegisterScenario registers a scenario under a kebab-case name that
// x-orcon-examples refer to. It is meant to be called from init functions and
// panics on an empty name, a nil function or a duplicate name.
func RegisterScenario(name string, s Scenario) {
	if name == "" || s == nil {
		panic("examplestest: RegisterScenario needs a name and a function")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("examplestest: scenario %q is registered twice", name))
	}
	registry[name] = s
}

// registered returns a copy of the registered scenarios.
func registered() map[string]Scenario {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make(map[string]Scenario, len(registry))
	for k, v := range registry {
		out[k] = v
	}
	return out
}

// Kind classifies why an example failed.
type Kind string

// The failure kinds. A passing example has the empty kind.
const (
	// KindScenario: the scenario is not registered or failed.
	KindScenario Kind = "scenario"
	// KindUnbound: a path parameter carries an id that no binding resolves.
	KindUnbound Kind = "unbound-id"
	// KindRequest: the request could not be built or sent.
	KindRequest Kind = "request"
	// KindStatus: the response status is not the one the example declares.
	KindStatus Kind = "status"
	// KindEnvelope: the response is wrapped in an envelope object.
	KindEnvelope Kind = "envelope"
	// KindSchema: the body does not validate against the response schema.
	KindSchema Kind = "schema"
	// KindValue: a leaf of the example differs from the real body.
	KindValue Kind = "value"
	// KindOperation: a long-running operation did not reach the expected status.
	KindOperation Kind = "operation"
	// KindStream: an event stream did not deliver the expected events.
	KindStream Kind = "stream"
	// KindContract: the example itself is malformed or references something absent.
	KindContract Kind = "contract"
)

// Result is the outcome of one example.
type Result struct {
	// Contract is the contract file name used in messages.
	Contract string
	// OperationID and Example identify the case.
	OperationID string
	Example     string
	// DB is the database engine the server ran on.
	DB string
	// Kind is empty when the example passed.
	Kind Kind
	// Pointer is the JSON pointer of the difference inside the body, when there is one.
	Pointer string
	// Detail explains the failure.
	Detail string
}

// Failed reports whether the example failed.
func (r Result) Failed() bool { return r.Kind != "" }

// String renders the failure with the contract file, operationId, example name,
// database engine and JSON pointer of the difference.
func (r Result) String() string {
	if !r.Failed() {
		return fmt.Sprintf("%s: operationId=%s example=%s db=%s: ok", r.Contract, r.OperationID, r.Example, r.DB)
	}
	at := ""
	if r.Pointer != "" {
		at = " at " + r.Pointer
	}
	return fmt.Sprintf("%s: operationId=%s example=%s db=%s: %s: %s%s", r.Contract, r.OperationID, r.Example, r.DB, r.Kind, r.Detail, at)
}

// RunExamples runs every example of cfg's contract and reports each failure on
// tb, naming contract file, operationId, example, database engine and the JSON
// pointer of the difference. A contract with no example passes: the rule that
// every stable operation has one belongs to the contract lint.
func RunExamples(tb testing.TB, cfg Config) {
	tb.Helper()
	results, err := Run(tb.Context(), cfg)
	if err != nil {
		tb.Fatalf("examples: %v", err)
	}
	failed := 0
	for _, r := range results {
		if r.Failed() {
			failed++
			tb.Error(r.String())
		}
	}
	tb.Logf("examples: %d run on %s, %d failed", len(results), cfg.DB, failed)
}

// Run executes the examples and returns one Result per example, in contract
// order. The error is for setup problems (an unreadable contract), never for a
// failing example.
func Run(ctx context.Context, cfg Config) ([]Result, error) {
	r, err := newRunner(cfg)
	if err != nil {
		return nil, err
	}
	defs, err := r.contract.examples()
	if err != nil {
		return nil, err
	}
	r.byName = map[string][]exampleDef{}
	for i := range defs {
		r.byName[defs[i].Name] = append(r.byName[defs[i].Name], defs[i])
	}
	results := make([]Result, 0, len(defs))
	for i := range defs {
		d := &defs[i]
		res := Result{Contract: cfg.ContractName, OperationID: d.OperationID, Example: d.Name, DB: cfg.DB}
		if f := r.runExample(ctx, *d); f != nil {
			res.Kind, res.Pointer, res.Detail = f.kind, f.pointer, f.detail
		}
		results = append(results, res)
	}
	return results, nil
}

// sortedKeys returns the keys of m in order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
