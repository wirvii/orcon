// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package mockprovider is a simulated HTTP provider for end-to-end tests of the
// declarative interpreter and of the core HTTP node (SPEC-003 Enmienda 7, §7-4).
// It serves five paging styles, a write endpoint with injected faults, and a
// bearer check, and it records every request it receives so a test can assert
// what was sent and how often. It is a test helper: it is never linked into the
// product binary.
package mockprovider

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Token is the bearer token the provider accepts; any other value gets 401.
const Token = "mock-token"

// ItemCount is the number of items the paged listings serve.
const ItemCount = 25

// pageSize is the page size of the cursor, link and next-URL listings.
const pageSize = 10

// Record is one request the provider received.
type Record struct {
	Method         string
	Path           string
	RawQuery       string
	IdempotencyKey string
	Authorization  string
}

// Provider is the handler plus its request log.
type Provider struct {
	mu       sync.Mutex
	requests []Record
	keys     map[string]int
}

// New returns a provider with an empty log.
func New() *Provider { return &Provider{keys: map[string]int{}} }

// Requests returns a copy of the log in arrival order.
func (p *Provider) Requests() []Record {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Record(nil), p.requests...)
}

// AppliedWrites returns how many times the write endpoint applied the writes
// carrying the idempotency key. A key applied twice is a duplicate effect.
func (p *Provider) AppliedWrites(key string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.keys[key]
}

// ServeHTTP implements http.Handler.
func (p *Provider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	p.requests = append(p.requests, Record{
		Method:         r.Method,
		Path:           r.URL.Path,
		RawQuery:       r.URL.RawQuery,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
		Authorization:  r.Header.Get("Authorization"),
	})
	p.mu.Unlock()

	if r.Header.Get("Authorization") != "Bearer "+Token {
		writeJSON(w, http.StatusUnauthorized, `{"error":"invalid token"}`)
		return
	}
	switch r.URL.Path {
	case "/v1/items":
		writeJSON(w, http.StatusOK, `{"data":`+itemsText(intParam(r, "offset", 0), intParam(r, "limit", pageSize))+`}`)
	case "/v1/pages":
		page := intParam(r, "page", 1)
		writeJSON(w, http.StatusOK, `{"data":`+itemsText((page-1)*pageSize, pageSize)+`}`)
	case "/v1/cursor":
		start := intParam(r, "cursor", 0)
		body := `{"data":` + itemsText(start, pageSize)
		if start+pageSize < ItemCount {
			body += fmt.Sprintf(`,"next":"%d"`, start+pageSize)
		}
		writeJSON(w, http.StatusOK, body+`}`)
	case "/v1/link":
		start := intParam(r, "start", 0)
		if start+pageSize < ItemCount {
			w.Header().Set("Link", fmt.Sprintf(`</v1/link?start=%d>; rel="next"`, start+pageSize))
		}
		writeJSON(w, http.StatusOK, itemsText(start, pageSize))
	case "/v1/next":
		start := intParam(r, "start", 0)
		body := `{"data":` + itemsText(start, pageSize)
		if start+pageSize < ItemCount {
			body += fmt.Sprintf(`,"nextUrl":"/v1/next?start=%d"`, start+pageSize)
		}
		writeJSON(w, http.StatusOK, body+`}`)
	case "/v1/things":
		p.write(w, r)
	default:
		writeJSON(w, http.StatusNotFound, `{"error":"no such resource"}`)
	}
}

// write applies a write unless the request asks for a fault. The fault comes
// from the X-Mock-Fault header: "5xx" applies the write and answers 503,
// "429" answers rate limited without applying, and "hangup" applies the write
// and drops the connection before any answer.
func (p *Provider) write(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, `{"error":"POST only"}`)
		return
	}
	fault := r.Header.Get("X-Mock-Fault")
	if fault == "429" {
		w.Header().Set("Retry-After", "0")
		writeJSON(w, http.StatusTooManyRequests, `{"error":"slow down"}`)
		return
	}
	p.mu.Lock()
	p.keys[r.Header.Get("Idempotency-Key")]++
	p.mu.Unlock()
	if fault == "hangup" {
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				_ = conn.Close()
				return
			}
		}
	}
	if fault == "5xx" {
		writeJSON(w, http.StatusServiceUnavailable, `{"error":"try later"}`)
		return
	}
	writeJSON(w, http.StatusCreated, `{"id":"thing-1","accepted":true}`)
}

// itemsText is a JSON array of the items from offset, at most limit long.
func itemsText(offset, limit int) string {
	var b strings.Builder
	b.WriteString("[")
	for i := offset; i < offset+limit && i < ItemCount; i++ {
		if i > offset {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":%d,"name":"item-%d"}`, i, i)
	}
	b.WriteString("]")
	return b.String()
}

func intParam(r *http.Request, name string, def int) int {
	if v := r.URL.Query().Get(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
