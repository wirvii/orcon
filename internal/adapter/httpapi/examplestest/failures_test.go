// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// miniContract holds a stream route and a trigger route for the stream tests.
const miniContract = `{"openapi":"3.1.0","paths":{
 "/s":{"get":{"operationId":"s","responses":{"200":{"description":"stream"}}}},
 "/t":{"post":{"operationId":"t","responses":{"204":{"description":"done"}}}}}}`

func newTestRunner(t *testing.T, srv *httptest.Server) *runner {
	t.Helper()
	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	r, err := newRunner(Config{
		Contract: []byte(miniContract), BaseURL: srv.URL, HTTPClient: &http.Client{Transport: transport},
		PollInterval: 5 * time.Millisecond, OperationTimeout: 80 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("newRunner: %v", err)
	}
	return r
}

func TestAwaitOperationFailures(t *testing.T) {
	// The handler runs on server goroutines, so the status it reports is
	// shared through an atomic value to keep the race detector quiet.
	var status atomic.Value
	status.Store("RUNNING")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		current, _ := status.Load().(string)
		_, _ = fmt.Fprintf(w, `{"id":"x","status":%q}`, current)
	}))
	defer srv.Close()
	r := newTestRunner(t, srv)
	def := exampleDef{Operation: &operationDef{TerminalStatus: "SUCCEEDED"}}
	located := &Response{Status: http.StatusAccepted, Header: http.Header{"Location": {"/op/1"}}}

	status.Store("FAILED")
	if f := r.awaitOperation(t.Context(), def, located); f == nil || f.kind != KindOperation || !strings.Contains(f.detail, "ended FAILED") {
		t.Errorf("wrong terminal status: %+v", f)
	}
	status.Store("RUNNING")
	if f := r.awaitOperation(t.Context(), def, located); f == nil || f.kind != KindOperation || !strings.Contains(f.detail, "not terminal") {
		t.Errorf("timeout: %+v", f)
	}
	if f := r.awaitOperation(t.Context(), def, &Response{Status: http.StatusAccepted, Header: http.Header{}}); f == nil || !strings.Contains(f.detail, "no Location") {
		t.Errorf("missing Location: %+v", f)
	}
	status.Store("SUCCEEDED")
	if f := r.awaitOperation(t.Context(), def, located); f != nil {
		t.Errorf("the expected terminal status failed: %+v", f)
	}
}

func TestRunStreamFailures(t *testing.T) {
	var events, contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, events)
		_ = http.NewResponseController(w).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	r := newTestRunner(t, srv)
	r.byName = map[string][]exampleDef{"t": {{
		Name: "t", Path: "/t", Method: http.MethodPost, Status: http.StatusNoContent,
		Op: map[string]any{"responses": map[string]any{"204": map[string]any{}}},
	}}}
	def := exampleDef{
		Name: "s", Path: "/s", Method: http.MethodGet, Status: http.StatusOK,
		Stream: &streamDef{Trigger: "t", ExpectTypes: []string{"a.first", "b.second"}, Within: 150 * time.Millisecond},
	}
	frame := func(types ...string) string {
		var b strings.Builder
		for _, typ := range types {
			b.WriteString("event: " + typ + "\ndata: {}\n\n")
		}
		return b.String()
	}

	contentType, events = "text/event-stream", frame("a.first", "b.second")
	if f := r.runStream(t.Context(), def, Bindings{}); f != nil {
		t.Fatalf("the expected events failed: %+v", f)
	}
	contentType, events = "text/event-stream", frame("b.second", "a.first")
	if f := r.runStream(t.Context(), def, Bindings{}); f == nil || f.kind != KindStream || !strings.Contains(f.detail, "in order") {
		t.Errorf("wrong order: %+v", f)
	}
	contentType, events = "text/event-stream", ""
	if f := r.runStream(t.Context(), def, Bindings{}); f == nil || f.kind != KindStream {
		t.Errorf("timeout: %+v", f)
	}
	contentType, events = "text/plain", frame("a.first", "b.second")
	if f := r.runStream(t.Context(), def, Bindings{}); f == nil || f.kind != KindStream || !strings.Contains(f.detail, "text/event-stream") {
		t.Errorf("wrong content type: %+v", f)
	}
}
