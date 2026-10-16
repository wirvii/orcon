// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package conformancetest is the common conformance kit of declarative and
// native connectors (ADR-0031 §5, ADR-0079 §3). A [Cassette] is a recorded
// exchange: the call, the provider's answers in order, and the expected result.
// [RunCassette] replays it offline through a [declarative.Sender] and checks the
// base sections every connector must pass: the request matches the recording,
// the items match the expected items, no credential value reaches an item, and
// the number of requests is exactly the number recorded.
//
// The kit is an area package: it imports no network or format package. Cassette
// files are decoded by the caller with the codec adapters.
package conformancetest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Cassette is one recorded case of an operation.
type Cassette struct {
	Name        string
	Operation   string
	Parameters  map[string]jsonvalue.Value
	Credentials map[string]string
	// Interactions are the provider exchanges, in the order they happen. A
	// request that does not match the next one fails the case.
	Interactions []Interaction
	// Items are the expected items, compared in canonical JSON form.
	Items []jsonvalue.Value
	// Failure, when set, is the expected failure instead of Items.
	Failure *ExpectedFailure
}

// Interaction is one request the connector must send and the answer recorded
// for it. When Err is set the sender returns that error instead of a response.
type Interaction struct {
	Method string
	URL    string
	// Header lists header values the request must carry exactly.
	Header   map[string]string
	Response declarative.HTTPResponse
	Err      error
}

// ExpectedFailure names the class and code a failing case must produce.
type ExpectedFailure struct {
	Class declarative.Class
	Code  string
}

// RunCassette replays c against in and reports every violated base section.
func RunCassette(tb testing.TB, in *declarative.Interpreter, c Cassette) {
	tb.Helper()
	sender := &replay{t: tb, want: c.Interactions}
	in2, err := rebind(in, sender)
	if err != nil {
		tb.Fatalf("%s: %v", c.Name, err)
	}
	items, err := collect(in2, c)
	sent := sender.next
	if sent != len(c.Interactions) {
		tb.Errorf("%s: sent %d requests, the recording has %d", c.Name, sent, len(c.Interactions))
	}
	if c.Failure != nil {
		var f *declarative.CallError
		if !errors.As(err, &f) {
			tb.Fatalf("%s: want failure %s/%s, got %v", c.Name, c.Failure.Class, c.Failure.Code, err)
		}
		if f.Class != c.Failure.Class || f.Code != c.Failure.Code {
			tb.Errorf("%s: failure %s/%s, want %s/%s", c.Name, f.Class, f.Code, c.Failure.Class, c.Failure.Code)
		}
		return
	}
	if err != nil {
		tb.Fatalf("%s: unexpected failure: %v", c.Name, err)
	}
	compareItems(tb, c, items)
}

// rebind creates a fresh interpreter over the same definition with the replay
// sender, so one definition can run many cassettes.
func rebind(in *declarative.Interpreter, s declarative.Sender) (*declarative.Interpreter, error) {
	return in.WithSender(s)
}

func collect(in *declarative.Interpreter, c Cassette) ([]jsonvalue.Value, error) {
	res, err := in.Execute(context.Background(), declarative.Call{
		Operation:   c.Operation,
		Parameters:  c.Parameters,
		Credentials: c.Credentials,
	}, nil)
	return res.Items, err
}

func compareItems(tb testing.TB, c Cassette, items []jsonvalue.Value) {
	tb.Helper()
	if len(items) != len(c.Items) {
		tb.Errorf("%s: got %d items, want %d", c.Name, len(items), len(c.Items))
	}
	for i := range min(len(items), len(c.Items)) {
		got, err1 := jsonvalue.Canonical(items[i])
		want, err2 := jsonvalue.Canonical(c.Items[i])
		if err1 != nil || err2 != nil {
			tb.Fatalf("%s: item %d is not encodable", c.Name, i)
		}
		if !bytes.Equal(got, want) {
			tb.Errorf("%s: item %d is %s, want %s", c.Name, i, got, want)
		}
		for _, secret := range c.Credentials {
			if secret != "" && strings.Contains(string(got), secret) {
				tb.Errorf("%s: item %d carries a credential value", c.Name, i)
			}
		}
	}
}

// replay answers requests from a recording, in order.
type replay struct {
	t    testing.TB
	want []Interaction
	next int
}

func (r *replay) Send(_ context.Context, req declarative.HTTPRequest) (declarative.HTTPResponse, error) {
	if r.next >= len(r.want) {
		r.t.Errorf("unexpected request %d: %s", r.next+1, req.Method)
		return declarative.HTTPResponse{}, fmt.Errorf("%w: no recorded answer", declarative.ErrNotSent)
	}
	want := r.want[r.next]
	r.next++
	if req.Method != want.Method || req.URL != want.URL {
		r.t.Errorf("request %d is %s %s, the recording has %s %s", r.next, req.Method, req.URL, want.Method, want.URL)
	}
	for k, v := range want.Header {
		got := req.Header[k]
		if len(got) == 0 || got[0] != v {
			r.t.Errorf("request %d header %s differs from the recording", r.next, k)
		}
	}
	if want.Err != nil {
		return declarative.HTTPResponse{}, want.Err
	}
	return want.Response, nil
}
