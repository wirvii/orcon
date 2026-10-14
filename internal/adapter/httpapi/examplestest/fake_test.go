// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/examplestest"
)

// fakeServer is a minimal server for testdata/fake-contract.json. Resources are
// returned bare, so every planted fault comes from the contract's examples.
type fakeServer struct {
	mu     sync.Mutex
	things map[string]string
	seq    int
	subs   map[chan string]struct{}
	polls  map[string]int
}

func newFakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	f := &fakeServer{things: map[string]string{}, subs: map[chan string]struct{}{}, polls: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/things", f.createThing)
	mux.HandleFunc("GET /api/v1/things/stream", f.streamThings)
	mux.HandleFunc("GET /api/v1/things/{thingId}", f.getThing)
	mux.HandleFunc("GET /api/v1/broken", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"id": "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60"})
	})
	mux.HandleFunc("GET /api/v1/wrapped", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": thingBody("0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60", "Alpha")})
	})
	mux.HandleFunc("POST /api/v1/imports", f.startImport)
	mux.HandleFunc("GET /api/v1/operations/{operationId}", f.getOperation)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func thingBody(id, name string) map[string]any {
	return map[string]any{"id": id, "name": name, "createdAt": time.Now().UTC().Format(time.RFC3339)}
}

func writeJSON(w http.ResponseWriter, status int, v map[string]any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (f *fakeServer) nextID() string {
	f.seq++
	return fmt.Sprintf("0191f6a2-3c4d-7e8f-9a0b-%012x", 0x1000+f.seq)
}

func (f *fakeServer) createThing(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	id := f.nextID()
	f.things[id] = in.Name
	for ch := range f.subs {
		select {
		case ch <- "thing.created":
		default:
		}
	}
	f.mu.Unlock()
	writeJSON(w, http.StatusCreated, thingBody(id, in.Name))
}

func (f *fakeServer) getThing(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	name, ok := f.things[r.PathValue("thingId")]
	f.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, thingBody(r.PathValue("thingId"), name))
}

func (f *fakeServer) streamThings(w http.ResponseWriter, r *http.Request) {
	ch := make(chan string, 8)
	f.mu.Lock()
	f.subs[ch] = struct{}{}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		delete(f.subs, ch)
		f.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	_ = rc.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case typ := <-ch:
			_, _ = fmt.Fprintf(w, "event: %s\ndata: {}\n\n", typ)
			_ = rc.Flush()
		}
	}
}

func (f *fakeServer) startImport(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	id := f.nextID()
	f.polls[id] = 0
	f.mu.Unlock()
	w.Header().Set("Location", "/api/v1/operations/"+id)
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "status": "PENDING"})
}

func (f *fakeServer) getOperation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("operationId")
	f.mu.Lock()
	n, ok := f.polls[id]
	f.polls[id] = n + 1
	f.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	status := "RUNNING"
	if n >= 2 {
		status = "SUCCEEDED"
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": status})
}

// seedThing creates a thing through the public API and binds the fixed id of the examples to it.
func seedThing(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	resp, err := api.Do(ctx, examplestest.Request{Method: http.MethodPost, Path: "/things", Body: map[string]any{"name": "Alpha"}})
	if err != nil {
		return nil, fmt.Errorf("seed thing: %w", err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := resp.Decode(&created); err != nil {
		return nil, fmt.Errorf("seed thing: %w", err)
	}
	return examplestest.Bindings{"0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60": created.ID}, nil
}

func fakeConfig(t *testing.T, srv *httptest.Server, contract []byte) examplestest.Config {
	t.Helper()
	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	return examplestest.Config{
		ContractName:     "testdata/fake-contract.json",
		Contract:         contract,
		BaseURL:          srv.URL,
		DB:               "sqlite",
		Scenarios:        map[string]examplestest.Scenario{"seed-thing": seedThing},
		HTTPClient:       &http.Client{Transport: transport, Timeout: 10 * time.Second},
		PollInterval:     5 * time.Millisecond,
		OperationTimeout: 5 * time.Second,
	}
}

// TestRunnerAgainstFake proves the runner without the binary: the good examples
// pass and each planted fault fails with its own reason.
func TestRunnerAgainstFake(t *testing.T) {
	contract := readFile(t, "testdata/fake-contract.json")
	srv := newFakeServer(t)
	results, err := examplestest.Run(t.Context(), fakeConfig(t, srv, contract))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	type want struct {
		kind    examplestest.Kind
		pointer string
		detail  string
	}
	wants := map[string]want{
		"getThing/good":             {},
		"createThing/createThing":   {},
		"streamThings/followThings": {},
		"startImport/importThings":  {},
		"getThing/wrongStatus":      {kind: examplestest.KindStatus, detail: "want status 201, got 200"},
		"getBroken/badBody":         {kind: examplestest.KindSchema, detail: "does not validate"},
		"getThing/wrongValue":       {kind: examplestest.KindValue, pointer: "/name", detail: `want "Zeta", got "Alpha"`},
		"getThing/missingScenario":  {kind: examplestest.KindScenario, detail: `"no-such-scenario" is not registered`},
		"getThing/unboundId":        {kind: examplestest.KindUnbound, detail: "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a61"},
		"getWrapped/envelope":       {kind: examplestest.KindEnvelope, detail: "envelope"},
	}
	if len(results) != len(wants) {
		t.Fatalf("got %d results, want %d (the protocol example must not run): %v", len(results), len(wants), results)
	}
	for _, r := range results {
		key := r.OperationID + "/" + r.Example
		w, ok := wants[key]
		if !ok {
			t.Errorf("unexpected example %s", key)
			continue
		}
		if r.Kind != w.kind {
			t.Errorf("%s: kind %q, want %q (%s)", key, r.Kind, w.kind, r)
			continue
		}
		if w.kind == "" {
			continue
		}
		if r.Pointer != w.pointer {
			t.Errorf("%s: pointer %q, want %q", key, r.Pointer, w.pointer)
		}
		if !strings.Contains(r.Detail, w.detail) {
			t.Errorf("%s: detail %q does not contain %q", key, r.Detail, w.detail)
		}
		for _, part := range []string{"testdata/fake-contract.json", "operationId=" + r.OperationID, "example=" + r.Example, "db=sqlite"} {
			if !strings.Contains(r.String(), part) {
				t.Errorf("%s: message %q does not contain %q", key, r.String(), part)
			}
		}
	}
}

// TestRunExamplesPassesGoodContract runs the exported entry point on a contract
// with only the good examples.
func TestRunExamplesPassesGoodContract(t *testing.T) {
	contract := goodOnly(t, readFile(t, "testdata/fake-contract.json"))
	srv := newFakeServer(t)
	examplestest.RunExamples(t, fakeConfig(t, srv, contract))
}

// goodOnly drops every example that is planted to fail.
func goodOnly(t *testing.T, raw []byte) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse fake contract: %v", err)
	}
	planted := map[string]bool{"wrongStatus": true, "wrongValue": true, "unboundId": true, "missingScenario": true, "badBody": true, "envelope": true}
	paths, _ := doc["paths"].(map[string]any)
	for _, itemAny := range paths {
		item, _ := itemAny.(map[string]any)
		for _, opAny := range item {
			op, _ := opAny.(map[string]any)
			list, _ := op["x-orcon-examples"].([]any)
			var kept []any
			for _, e := range list {
				entry, _ := e.(map[string]any)
				if name, _ := entry["name"].(string); !planted[name] {
					kept = append(kept, e)
				}
			}
			if list != nil {
				op["x-orcon-examples"] = kept
			}
		}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("encode contract: %v", err)
	}
	return out
}
