// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package mockllm is a scripted HTTP model provider for tests (ADR-0061 §7,
// R-PRV-08). A test gives it an ordered list of [Reply] values; each request
// receives the next reply, which can be a normal answer, a status code with
// headers, a delay, or a dropped connection. Every request is recorded so the
// test can check what an adapter sent, including that no secret value leaked
// into a body.
//
// It serves whatever path the adapter calls; the wire format of each reply is
// the test's responsibility. It is a test helper and is never linked into the
// product binary.
package mockllm

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Reply is one scripted answer.
type Reply struct {
	// Status is the HTTP status code. Zero means 200.
	Status int
	// Header holds extra response headers, for example Retry-After.
	Header http.Header
	// Body is written verbatim.
	Body string
	// Delay holds the answer back, honoring the request context.
	Delay time.Duration
	// Hangup closes the connection without writing a response, which the
	// adapter must see as a transient transport failure.
	Hangup bool
}

// Request is one request the mock received.
type Request struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// Server is a running scripted provider.
type Server struct {
	t       testing.TB
	srv     *httptest.Server
	mu      sync.Mutex
	script  []Reply
	next    int
	records []Request
}

// New starts a server that answers with replies in order. Requests beyond the
// script fail the test and receive a 500, so a test that makes one call too
// many is caught. The server is closed when the test ends.
func New(tb testing.TB, replies ...Reply) *Server {
	tb.Helper()
	s := &Server{t: tb, script: append([]Reply(nil), replies...)}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	tb.Cleanup(s.srv.Close)
	return s
}

// URL returns the base URL of the server, without a trailing slash.
func (s *Server) URL() string { return s.srv.URL }

// Client returns an HTTP client that reaches the server. Adapters receive it
// instead of the egress client, which refuses loopback addresses.
func (s *Server) Client() *http.Client { return s.srv.Client() }

// Requests returns a copy of every request received so far, in order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, len(s.records))
	copy(out, s.records)
	return out
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.records = append(s.records, Request{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone(), Body: body})
	if s.next >= len(s.script) {
		s.mu.Unlock()
		s.t.Errorf("mockllm: request %d (%s %s) has no scripted reply", s.next+1, r.Method, r.URL.Path)
		http.Error(w, "script exhausted", http.StatusInternalServerError)
		return
	}
	reply := s.script[s.next]
	s.next++
	s.mu.Unlock()

	if reply.Delay > 0 {
		select {
		case <-time.After(reply.Delay):
		case <-r.Context().Done():
			return
		}
	}
	if reply.Hangup {
		hijackAndClose(w)
		return
	}
	for k, vs := range reply.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	status := reply.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = io.WriteString(w, reply.Body)
}

func hijackAndClose(w http.ResponseWriter) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	_ = conn.Close()
}
