// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package scenarios

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"

	"github.com/wirvii/orcon/internal/adapter/httpapi/examplestest"
)

// ErrNotSeedable means a scenario needs an engine capability that does not
// exist yet; the examples that name it cannot run until then.
var ErrNotSeedable = errors.New("the state cannot be seeded through the public API yet")

var keyCounter atomic.Uint64

// seeder runs the calls of one scenario and remembers the first failure.
type seeder struct {
	ctx context.Context //nolint:containedctx // a seeder lives for one scenario and carries its context
	api *examplestest.Client
	b   examplestest.Bindings
	err error
}

func newSeeder(ctx context.Context, api *examplestest.Client) *seeder {
	return &seeder{ctx: ctx, api: api, b: examplestest.Bindings{fixedOrg: api.OrgID()}}
}

// result returns the bindings and the first error.
func (s *seeder) result() (examplestest.Bindings, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.b, nil
}

// call sends a request, requires status want and decodes the JSON body into out when it is not nil.
func (s *seeder) call(req examplestest.Request, want int, out any) {
	if s.err != nil {
		return
	}
	resp, err := s.api.Do(s.ctx, req)
	if err != nil {
		s.err = fmt.Errorf("seed %s %s: %w", req.Method, req.Path, err)
		return
	}
	if resp.Status != want {
		s.err = fmt.Errorf("seed %s %s: status %d, want %d: %s", req.Method, req.Path, resp.Status, want, string(resp.Body))
		return
	}
	if out != nil {
		if err := resp.Decode(out); err != nil {
			s.err = fmt.Errorf("seed %s %s: %w", req.Method, req.Path, err)
		}
	}
}

func key(prefix string) http.Header {
	h := http.Header{}
	h.Set("Idempotency-Key", prefix+"-"+strconv.FormatUint(keyCounter.Add(1), 10))
	return h
}

func (s *seeder) project(name, slug, fixed string) string {
	var p struct {
		ID string `json:"id"`
	}
	s.call(examplestest.Request{
		Method: http.MethodPost, Path: "/orgs/" + s.api.OrgID() + "/projects", Header: key("seed-project"),
		Body: map[string]any{"name": name, "slug": slug, "environment": "prod"},
	}, http.StatusCreated, &p)
	s.bind(fixed, p.ID)
	return p.ID
}

func (s *seeder) flow(projectID, name, fixed string) string {
	var f struct {
		ID string `json:"id"`
	}
	s.call(examplestest.Request{
		Method: http.MethodPost, Path: "/projects/" + projectID + "/flows", Header: key("seed-flow"),
		Body: map[string]any{"name": name},
	}, http.StatusCreated, &f)
	s.bind(fixed, f.ID)
	return f.ID
}

// saveDraft replaces the draft with document; revision is the draft revision the caller expects.
func (s *seeder) saveDraft(flowID string, revision int, document map[string]any) {
	h := http.Header{}
	h.Set("If-Match", `"`+strconv.Itoa(revision)+`"`)
	s.call(examplestest.Request{
		Method: http.MethodPut, Path: "/flows/" + flowID + "/draft", Header: h,
		Body: map[string]any{"document": document},
	}, http.StatusOK, nil)
}

func (s *seeder) publish(flowID string, revision int) {
	h := key("seed-publish")
	h.Set("If-Match", `"`+strconv.Itoa(revision)+`"`)
	s.call(examplestest.Request{
		Method: http.MethodPost, Path: "/flows/" + flowID + "/publish", Header: h, Body: map[string]any{},
	}, http.StatusOK, nil)
}

// activate switches the flow on; revision is the revision of the flow row, which
// publishing raised to 1 (Amendment 8, 8-9).
func (s *seeder) activate(flowID string, revision int) {
	h := key("seed-activate")
	h.Set("If-Match", `"`+strconv.Itoa(revision)+`"`)
	s.call(examplestest.Request{Method: http.MethodPost, Path: "/flows/" + flowID + "/activate", Header: h}, http.StatusOK, nil)
}

// run starts an execution and binds its fixed id; it accepts 200 and 202 so a
// scenario can seed both finished and still-running executions.
func (s *seeder) run(flowID string, wait int, fixed string) string {
	var e struct {
		ID string `json:"id"`
	}
	if s.err != nil {
		return ""
	}
	resp, err := s.api.Do(s.ctx, examplestest.Request{
		Method: http.MethodPost, Path: "/flows/" + flowID + "/executions", Header: key("seed-run"),
		Query: url.Values{"wait": {strconv.Itoa(wait)}}, Body: map[string]any{},
	})
	if err != nil {
		s.err = fmt.Errorf("seed run: %w", err)
		return ""
	}
	if resp.Status != http.StatusOK && resp.Status != http.StatusAccepted {
		s.err = fmt.Errorf("seed run: status %d: %s", resp.Status, string(resp.Body))
		return ""
	}
	if err := resp.Decode(&e); err != nil {
		s.err = fmt.Errorf("seed run: %w", err)
		return ""
	}
	s.bind(fixed, e.ID)
	return e.ID
}

// firstNode binds the fixed node execution id to the first node execution of an execution.
func (s *seeder) firstNode(executionID, fixed string) {
	var l struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	s.call(examplestest.Request{Method: http.MethodGet, Path: "/executions/" + executionID + "/nodes"}, http.StatusOK, &l)
	if s.err == nil && len(l.Items) > 0 {
		s.bind(fixed, l.Items[0].ID)
	}
}

func (s *seeder) bind(fixed, actual string) {
	if s.err == nil && fixed != "" && actual != "" {
		s.b[fixed] = actual
	}
}

// Flow documents. They use only nodes that need no network.

func baseDocument(name string, nodes []map[string]any, edges []map[string]any) map[string]any {
	return map[string]any{
		"format": "orcon/flow@1", "id": "01930000-0000-7000-8000-00000000000a", "name": name,
		"settings": map[string]any{"branchMode": "PARALLEL"}, "variables": []any{},
		"nodes": nodes, "edges": edges, "editor": map[string]any{},
	}
}

func node(id, name, element, typ string, extra map[string]any) map[string]any {
	n := map[string]any{"id": id, "name": name, "element": element, "type": typ, "parameters": map[string]any{}}
	for k, v := range extra {
		n[k] = v
	}
	return n
}

func edge(id, from, port, to string) map[string]any {
	return map[string]any{"id": id, "from": map[string]any{"node": from, "port": port}, "to": map[string]any{"node": to}, "kind": "SEQUENCE"}
}

// greetingDocument is start, one task and end.
func greetingDocument(name string) map[string]any {
	return baseDocument(name,
		[]map[string]any{
			node("start", "Start", "startEvent", "orcon.start@1", nil),
			node("greeting", "Set greeting", "task", "orcon.set@1", nil),
			node("end", "End", "endEvent", "orcon.end@1", nil),
		},
		[]map[string]any{edge("e1", "start", "main", "greeting"), edge("e2", "greeting", "main", "end")},
	)
}

// signalWaitDocument parks on a signal wait with a timeout path.
func signalWaitDocument(name string) map[string]any {
	return baseDocument(name,
		[]map[string]any{
			node("start", "Start", "startEvent", "orcon.start@1", nil),
			node("callback", "Wait for callback", "receiveTask", "orcon.wait@1", map[string]any{"wait": map[string]any{"kind": "SIGNAL", "timeout": "PT1H"}}),
			node("end", "End", "endEvent", "orcon.end@1", nil),
			node("timed-out", "Timed out", "endEvent", "orcon.end@1", nil),
		},
		[]map[string]any{edge("e1", "start", "main", "callback"), edge("e2", "callback", "main", "end"), edge("e3", "callback", "timeout", "timed-out")},
	)
}

// humanTaskDocument parks on a human task with a timeout path.
func humanTaskDocument(name string) map[string]any {
	return baseDocument(name,
		[]map[string]any{
			node("start", "Start", "startEvent", "orcon.start@1", nil),
			node("approve", "Approve", "userTask", "orcon.humanTask@1", map[string]any{
				"parameters": map[string]any{"assignee": "role:reviewer"},
				"wait":       map[string]any{"kind": "HUMAN", "timeout": "P2D", "form": map[string]any{"type": "object", "properties": map[string]any{"approved": map[string]any{"type": "boolean"}}}},
			}),
			node("end", "End", "endEvent", "orcon.end@1", nil),
			node("gave-up", "Gave up", "endEvent", "orcon.end@1", nil),
		},
		[]map[string]any{edge("e1", "start", "main", "approve"), edge("e2", "approve", "main", "end"), edge("e3", "approve", "timeout", "gave-up")},
	)
}

// failingDocument fails at its task: the HTTP node asks for an address the
// egress policy refuses, so the failure needs no network.
func failingDocument(name string) map[string]any {
	return baseDocument(name,
		[]map[string]any{
			node("start", "Start", "startEvent", "orcon.start@1", nil),
			node("call", "Blocked call", "serviceTask", "orcon.http@1", map[string]any{"parameters": map[string]any{"url": "http://127.0.0.1:1/blocked"}}),
			node("end", "End", "endEvent", "orcon.end@1", nil),
		},
		[]map[string]any{edge("e1", "start", "main", "call"), edge("e2", "call", "main", "end")},
	)
}
