// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package mockprovider is a simulated HTTP provider for end-to-end tests of the
// declarative interpreter and of the core HTTP node (SPEC-003 Enmienda 7, §7-4).
// It serves five paging styles, a write endpoint with injected faults, a
// WhatsApp Cloud API style Graph endpoint (messages and phone numbers, under
// [GraphPrefix]) and a bearer check, and it records every request it receives
// so a test can assert what was sent and how often. It is a test helper: it is
// never linked into the product binary.
package mockprovider

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Token is the bearer token the provider accepts; any other value gets 401.
const Token = "mock-token"

// GraphPrefix is the path prefix of the Graph style endpoints. The version is
// part of the path, as in the WhatsApp Cloud API.
const GraphPrefix = "/v25.0/"

// MockPhoneNumberID is the phone number the Graph endpoints serve. Its ID is
// fixed, so no request data reaches a response.
const MockPhoneNumberID = "106540352242922"

// MockContact is the contact the Graph messages endpoint echoes back.
const MockContact = "573000000000"

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
	Body           []byte
}

// Provider is the handler plus its request log.
type Provider struct {
	mu       sync.Mutex
	requests []Record
	keys     map[string]int
	applied  int
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

// Applied returns how many writes the write endpoints applied, whatever their
// key. A write answered with a fault that never reached the provider is not
// counted.
func (p *Provider) Applied() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.applied
}

// ServeHTTP implements http.Handler.
func (p *Provider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
	}
	p.mu.Lock()
	p.requests = append(p.requests, Record{
		Method:         r.Method,
		Path:           r.URL.Path,
		RawQuery:       r.URL.RawQuery,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
		Authorization:  r.Header.Get("Authorization"),
		Body:           body,
	})
	p.mu.Unlock()

	if r.Header.Get("Authorization") != "Bearer "+Token {
		writeJSON(w, http.StatusUnauthorized, `{"error":{"message":"Invalid OAuth access token.","type":"OAuthException","code":190}}`)
		return
	}
	if strings.HasPrefix(r.URL.Path, GraphPrefix) {
		p.graph(w, r)
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
		p.write(w, r, func(int) (int, string) {
			return http.StatusCreated, `{"id":"thing-1","accepted":true}`
		})
	default:
		writeJSON(w, http.StatusNotFound, `{"error":"no such resource"}`)
	}
}

// graph serves the Graph style endpoints: GET <prefix><id> returns the phone
// number, POST <prefix><id>/messages sends a message.
func (p *Provider) graph(w http.ResponseWriter, r *http.Request) {
	_, tail, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, GraphPrefix), "/")
	switch {
	case tail == "" && r.Method == http.MethodGet:
		if refuseFault(w, r.Header.Get("X-Mock-Fault")) {
			return
		}
		writeJSON(w, http.StatusOK, fmt.Sprintf(`{"id":%q,"display_phone_number":"+1 555 0100","verified_name":"Mock Shop","quality_rating":"GREEN"}`, MockPhoneNumberID))
	case tail == "messages" && r.Method == http.MethodPost:
		p.write(w, r, func(seq int) (int, string) {
			return http.StatusOK, fmt.Sprintf(`{"messaging_product":"whatsapp","contacts":[{"input":%q,"wa_id":%q}],"messages":[{"id":"wamid.mock-%d"}]}`, MockContact, MockContact, seq)
		})
	default:
		writeJSON(w, http.StatusNotFound, `{"error":{"message":"Unsupported request","type":"GraphMethodException","code":100}}`)
	}
}

// write applies a write unless the request asks for a fault. The fault comes
// from the X-Mock-Fault header: "5xx" applies the write and answers 503,
// "429" answers rate limited without applying, "400" answers a validation error
// without applying, and "hangup" applies the write and drops the connection
// before any answer. The reply function builds the success answer from the
// number of the applied write.
func (p *Provider) write(w http.ResponseWriter, r *http.Request, reply func(seq int) (int, string)) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, `{"error":"POST only"}`)
		return
	}
	fault := r.Header.Get("X-Mock-Fault")
	if refuseFault(w, fault) {
		return
	}
	p.mu.Lock()
	p.keys[r.Header.Get("Idempotency-Key")]++
	p.applied++
	seq := p.applied
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
	status, body := reply(seq)
	writeJSON(w, status, body)
}

// refuseFault answers the faults that mean the request was not applied: "429"
// is rate limited and "400" is a validation error. It reports whether it wrote
// an answer.
func refuseFault(w http.ResponseWriter, fault string) bool {
	switch fault {
	case "429":
		w.Header().Set("Retry-After", "0")
		writeJSON(w, http.StatusTooManyRequests, `{"error":{"message":"Too many calls","type":"OAuthException","code":80007}}`)
		return true
	case "400":
		writeJSON(w, http.StatusBadRequest, `{"error":{"message":"(#100) Invalid parameter","type":"OAuthException","code":100}}`)
		return true
	}
	return false
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
