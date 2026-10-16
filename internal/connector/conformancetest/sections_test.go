// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package conformancetest

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// recorder is a testing.TB that collects the reports of a cassette run instead
// of failing the surrounding test. Fatalf ends the goroutine, as testing.T does.
type recorder struct {
	testing.TB
	reports []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.reports = append(r.reports, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	runtime.Goexit()
}

// runCase replays c against in and returns what the kit reported.
func runCase(t *testing.T, in *declarative.Interpreter, c Cassette) []string {
	t.Helper()
	rec := &recorder{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		RunCassette(rec, in, c)
	}()
	<-done
	return rec.reports
}

func expectReport(t *testing.T, reports []string, want string) {
	t.Helper()
	for _, r := range reports {
		if strings.Contains(r, want) {
			return
		}
	}
	t.Fatalf("no report mentions %q; reports = %q", want, reports)
}

func expectSilent(t *testing.T, reports []string) {
	t.Helper()
	if len(reports) != 0 {
		t.Fatalf("unexpected reports: %q", reports)
	}
}

// mustItem parses a JSON literal written by the test itself.
func mustItem(raw string) jsonvalue.Value {
	v, err := jsonvalue.Parse([]byte(raw))
	if err != nil {
		panic(fmt.Sprintf("bad test literal %s: %v", raw, err))
	}
	return v
}

func getCassette(name string, interactions ...Interaction) Cassette {
	return Cassette{
		Name:         name,
		Operation:    "item.get",
		Parameters:   map[string]jsonvalue.Value{"id": jsonvalue.String("7")},
		Credentials:  map[string]string{"token": "tok-secret"},
		Interactions: interactions,
		Items:        []jsonvalue.Value{mustItem(`{"id":"7"}`)},
	}
}

func getInteraction(status int, body string) Interaction {
	return Interaction{
		Method:   "GET",
		URL:      "https://api.example.com/items/7",
		Header:   map[string]string{"Authorization": "Bearer tok-secret"},
		Response: declarative.HTTPResponse{Status: status, Body: []byte(body)},
	}
}

func getInterpreter(t *testing.T, ops map[string]declarative.Operation) *declarative.Interpreter {
	t.Helper()
	def := definition()
	if ops != nil {
		def.Operations = ops
	}
	// Retries wait for nothing: the cases check classes, not timing.
	noWait := func(context.Context, time.Duration) error { return nil }
	in, err := declarative.New(def, declarative.Options{Sender: noSender{}, Sleep: noWait})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return in
}

func TestRunCassetteFlagsARequestThatDoesNotMatch(t *testing.T) {
	in := getInterpreter(t, nil)
	c := getCassette("wrong url", getInteraction(200, `{"data":{"id":"7"}}`))
	c.Interactions[0].URL = "https://api.example.com/items/8"
	// The mismatch is reported; the replayed answer still runs.
	expectReport(t, runCase(t, in, c), "the recording has GET https://api.example.com/items/8")
}

func TestRunCassetteFlagsAHeaderThatDiffers(t *testing.T) {
	in := getInterpreter(t, nil)
	c := getCassette("wrong header", getInteraction(200, `{"data":{"id":"7"}}`))
	c.Interactions[0].Header = map[string]string{"Authorization": "Bearer other"}
	expectReport(t, runCase(t, in, c), "header Authorization differs")
}

func TestRunCassetteFlagsUnusedRecordedRequests(t *testing.T) {
	in := getInterpreter(t, nil)
	c := getCassette("one request too many", getInteraction(200, `{"data":{"id":"7"}}`), getInteraction(200, `{"data":{"id":"7"}}`))
	expectReport(t, runCase(t, in, c), "sent 1 requests, the recording has 2")
}

func TestRunCassetteFlagsARequestWithNoRecordedAnswer(t *testing.T) {
	in := getInterpreter(t, nil)
	c := getCassette("nothing recorded")
	reports := runCase(t, in, c)
	expectReport(t, reports, "unexpected request 1")
	expectReport(t, reports, "unexpected failure")
}

func TestRunCassetteReportsATransportError(t *testing.T) {
	ops := map[string]declarative.Operation{
		"item.create": {
			Purity: declarative.PurityEffect, Safety: declarative.SafetyUnsafe,
			Request:  declarative.Request{Method: "POST", Path: "/items"},
			Response: declarative.ResponseSpec{},
		},
	}
	in := getInterpreter(t, ops)
	c := Cassette{
		Name:        "connection cut",
		Operation:   "item.create",
		Credentials: map[string]string{"token": "tok-secret"},
		Interactions: []Interaction{{
			Method: "POST",
			URL:    "https://api.example.com/items",
			Err:    errors.New("connection reset by peer"),
		}},
		Failure: &ExpectedFailure{Class: declarative.ClassUncertain, Code: "NetworkError"},
	}
	expectSilent(t, runCase(t, in, c))
}

func TestRunCassetteFlagsAFailureWithTheWrongClass(t *testing.T) {
	in := getInterpreter(t, nil)
	c := getCassette("expected transient", getInteraction(404, `{}`))
	c.Items = nil
	c.Failure = &ExpectedFailure{Class: declarative.ClassTransient, Code: "NotFound"}
	expectReport(t, runCase(t, in, c), "failure PERMANENT/NotFound, want TRANSIENT/NotFound")
}

func TestRunCassetteAcceptsTheExpectedFailure(t *testing.T) {
	in := getInterpreter(t, nil)
	c := getCassette("expected not found", getInteraction(404, `{}`))
	c.Items = nil
	c.Failure = &ExpectedFailure{Class: declarative.ClassPermanent, Code: "NotFound"}
	expectSilent(t, runCase(t, in, c))
}

func TestRunCassetteFlagsASuccessWhereAFailureWasExpected(t *testing.T) {
	in := getInterpreter(t, nil)
	c := getCassette("expected failure, got items", getInteraction(200, `{"data":{"id":"7"}}`))
	c.Failure = &ExpectedFailure{Class: declarative.ClassPermanent, Code: "NotFound"}
	expectReport(t, runCase(t, in, c), "want failure PERMANENT/NotFound")
}

func TestRunCassetteFlagsAnUnexpectedFailure(t *testing.T) {
	in := getInterpreter(t, nil)
	c := getCassette("expected items, got 404", getInteraction(404, `{}`))
	c.Items = nil
	expectReport(t, runCase(t, in, c), "unexpected failure")
}

func TestCompareItemsReportsCountAndContent(t *testing.T) {
	in := getInterpreter(t, nil)

	c := getCassette("two items expected, one returned", getInteraction(200, `{"data":{"id":"7"}}`))
	c.Items = append(c.Items, mustItem(`{"id":"8"}`))
	expectReport(t, runCase(t, in, c), "got 1 items, want 2")

	c = getCassette("different content", getInteraction(200, `{"data":{"id":"7"}}`))
	c.Items = []jsonvalue.Value{mustItem(`{"id":"9"}`)}
	expectReport(t, runCase(t, in, c), "item 0 is")
}

func TestCompareItemsFlagsCredentialInItem(t *testing.T) {
	in := getInterpreter(t, nil)
	c := getCassette("leaks the token", getInteraction(200, `{"data":{"id":"tok-secret"}}`))
	c.Items = []jsonvalue.Value{mustItem(`{"id":"tok-secret"}`)}
	expectReport(t, runCase(t, in, c), "carries a credential value")
}
