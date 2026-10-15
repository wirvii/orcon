// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package idempotency makes a POST safe to retry (ADR-0053 section 4): the
// first request with an Idempotency-Key runs, and a repeat with the same key and
// the same content gets the stored response with Idempotent-Replayed: true.
//
// The records live behind the [Store] port. A store is keyed by organization,
// exact requester and key, so two API keys of one user never share a record.
// Responses that disclose a freshly issued secret are stored without it: a
// replay answers 409 secret-already-issued with the address of the resource.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Header names of the convention.
const (
	// KeyHeader carries the client's idempotency key.
	KeyHeader = "Idempotency-Key"
	// ReplayedHeader marks a response served from a stored record.
	ReplayedHeader = "Idempotent-Replayed"
)

// Key limits (ADR-0053 section 4): 1 to 255 visible ASCII characters.
const (
	minKeyLength = 1
	maxKeyLength = 255
)

// maxStoredBody bounds a response the middleware keeps in memory to store; a
// longer one is sent but not stored, and its key is released.
const maxStoredBody = 1 << 20

// retryInProgress is the Retry-After of a request whose twin is still running.
const retryInProgress = time.Second

// Key identifies one idempotent request: who sent it and under which key.
type Key struct {
	// Org is the organization of the requester.
	Org string
	// Requester is the exact requesting identity: the user or service account for
	// sessions, the key or token for key-authenticated calls.
	Requester string
	// Value is the Idempotency-Key header.
	Value string
}

// Response is what a completed request answered, as far as a replay needs it.
type Response struct {
	// Status is the HTTP status.
	Status int
	// Header holds the headers a replay repeats: Content-Type, Location and ETag.
	Header http.Header
	// Body is the response body; empty when SecretIssued.
	Body []byte
	// SecretIssued is true when the response disclosed a secret that is not stored.
	SecretIssued bool
	// Unstored is true when a successful response was too large to keep: a repeat
	// is refused with 409 and the address of the resource, because the store has no
	// way yet to rebuild the answer from the resource reference (Amendment 8 section
	// 8-2); the durable store of API2-core replaces this with the rebuild.
	Unstored bool
}

// Record is the stored state of a key.
type Record struct {
	// Fingerprint is the SHA-256 of the request that claimed the key.
	Fingerprint [sha256.Size]byte
	// Completed is false while the first request is still running.
	Completed bool
	// Response is valid when Completed.
	Response Response
}

// Store keeps the records. Implementations are safe for concurrent use.
type Store interface {
	// Begin claims key for the request with fingerprint. When a record exists it
	// is returned and claimed is false; otherwise a record in progress is created
	// and claimed is true.
	Begin(ctx context.Context, key Key, fingerprint [sha256.Size]byte) (rec Record, claimed bool, err error)
	// Complete stores the response of a claimed key.
	Complete(ctx context.Context, key Key, resp *Response) error
	// Abandon releases a claimed key whose request produced no response worth storing.
	Abandon(ctx context.Context, key Key) error
}

// Config configures the middleware of one operation.
type Config struct {
	// Store keeps the records.
	Store Store
	// Logger receives store failures.
	Logger *slog.Logger
	// Required makes a missing key a 400 validation-failed.
	Required bool
	// IssuesSecret is true when a 2xx response of the operation carries a secret.
	IssuesSecret bool
	// Route is the route template, "POST /api/v1/flows/{flowId}/executions".
	Route string
}

// Middleware returns the idempotency step of the pipeline (step 7) for one
// operation. It must run after authentication.
func Middleware(cfg Config) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serve(&cfg, next, w, r)
		})
	}
}

func serve(cfg *Config, next http.Handler, w http.ResponseWriter, r *http.Request) {
	value := r.Header.Get(KeyHeader)
	if value == "" {
		if cfg.Required {
			problem.Write(w, r, problem.ValidationFailed, problem.Fields(problem.FieldError{
				Pointer: "/" + KeyHeader, Code: "required", Message: "The Idempotency-Key header is required.",
			}))
			return
		}
		next.ServeHTTP(w, r)
		return
	}
	if !validKey(value) {
		problem.Write(w, r, problem.ValidationFailed, problem.Fields(problem.FieldError{
			Pointer: "/" + KeyHeader, Code: "invalid-format", Message: "The key must be 1 to 255 visible ASCII characters.",
		}))
		return
	}
	principal, ok := middleware.PrincipalFrom(r.Context())
	if !ok {
		next.ServeHTTP(w, r)
		return
	}
	body, err := readBody(r)
	if err != nil {
		bodyError(w, r, err)
		return
	}
	fingerprint, err := Fingerprint(cfg.Route, pathValues(r), body)
	if err != nil {
		problem.Write(w, r, problem.ValidationFailed, problem.Detail("The body is not valid JSON."))
		return
	}
	key := Key{Org: principal.OrgID.String(), Requester: requesterOf(principal), Value: value}
	rec, claimed, err := cfg.Store.Begin(r.Context(), key, fingerprint)
	if err != nil {
		storeFailed(cfg, w, r, "begin", err)
		return
	}
	if !claimed {
		answerExisting(w, r, &rec, fingerprint)
		return
	}
	run(cfg, next, w, r, key)
}

// run executes the handler of a claimed key, stores the outcome and sends it.
func run(cfg *Config, next http.Handler, w http.ResponseWriter, r *http.Request, key Key) {
	rec := &recorder{header: http.Header{}, status: http.StatusOK}
	stored := false
	// The bookkeeping outlives a client that went away.
	ctx := context.WithoutCancel(r.Context())
	requestID := middleware.RequestIDFrom(ctx)
	defer func() {
		if !stored {
			// A panic or an early return: release the key so the client can retry.
			if err := cfg.Store.Abandon(ctx, key); err != nil {
				cfg.Logger.ErrorContext(ctx, "idempotency: abandon failed", "request_id", requestID, "error", err)
			}
		}
	}()
	next.ServeHTTP(rec, r)

	resp := rec.response(cfg.IssuesSecret)
	// Only a success is stored (Amendment 8 section 8-2(c)): a 4xx or a 5xx releases the
	// key, so the client can correct the request and send it again under the same key.
	if resp.Status < http.StatusOK || resp.Status >= http.StatusMultipleChoices {
		flush(w, rec)
		return
	}
	if rec.overflow {
		resp = &Response{Status: resp.Status, Header: resp.Header, Unstored: true}
	}
	if err := cfg.Store.Complete(ctx, key, resp); err != nil {
		cfg.Logger.ErrorContext(ctx, "idempotency: complete failed", "request_id", requestID, "error", err)
		flush(w, rec)
		return
	}
	stored = true
	flush(w, rec)
}

func answerExisting(w http.ResponseWriter, r *http.Request, rec *Record, fingerprint [sha256.Size]byte) {
	switch {
	case rec.Fingerprint != fingerprint:
		problem.Write(w, r, problem.IdempotencyKeyReused,
			problem.Detail("The Idempotency-Key was used with a different request."))
	case !rec.Completed:
		problem.Write(w, r, problem.IdempotencyInProgress, problem.RetryAfter(retryInProgress),
			problem.Detail("A request with this Idempotency-Key is still running."))
	case rec.Response.Unstored:
		opts := []problem.Option{problem.Detail("The response of the first request was too large to store; read the resource it created.")}
		if loc := rec.Response.Header.Get("Location"); loc != "" {
			opts = append(opts, problem.Header("Location", loc))
		}
		problem.Write(w, r, problem.Conflict, opts...)
	case rec.Response.SecretIssued:
		opts := []problem.Option{problem.Detail("The secret of this request was shown once and is not stored; read the resource instead.")}
		if loc := rec.Response.Header.Get("Location"); loc != "" {
			opts = append(opts, problem.Header("Location", loc))
		}
		problem.Write(w, r, problem.SecretAlreadyIssued, opts...)
	default:
		for name, values := range rec.Response.Header {
			for _, v := range values {
				w.Header().Add(name, v)
			}
		}
		w.Header().Set(ReplayedHeader, "true")
		w.WriteHeader(rec.Response.Status)
		if r.Method != http.MethodHead {
			_, _ = w.Write(rec.Response.Body)
		}
	}
}

func storeFailed(cfg *Config, w http.ResponseWriter, r *http.Request, step string, err error) {
	if errors.Is(err, ErrStoreUnavailable) {
		problem.Write(w, r, problem.ServiceUnavailable, problem.RetryAfter(retryInProgress))
		return
	}
	cfg.Logger.ErrorContext(r.Context(), "idempotency: store failed", "request_id", middleware.RequestIDFrom(r.Context()), "step", step, "error", err)
	problem.Write(w, r, problem.InternalError)
}

func bodyError(w http.ResponseWriter, r *http.Request, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		problem.Write(w, r, problem.PayloadTooLarge)
		return
	}
	problem.Write(w, r, problem.ValidationFailed, problem.Detail("The body could not be read."))
}

// readBody reads the whole body and puts it back for the handler.
func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read the request body: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return data, nil
}

// pathValues returns the path parameters of the matched route, sorted by name.
func pathValues(r *http.Request) []string {
	names := routeParams(r.Pattern)
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, n+"="+r.PathValue(n))
	}
	return out
}

// routeParams lists the {name} wildcards of a mux pattern.
func routeParams(pattern string) []string {
	var names []string
	for i := 0; i < len(pattern); i++ {
		if pattern[i] != '{' {
			continue
		}
		end := i + 1
		for end < len(pattern) && pattern[end] != '}' {
			end++
		}
		if end < len(pattern) {
			names = append(names, pattern[i+1:end])
		}
		i = end
	}
	return names
}

// Fingerprint is the SHA-256 of the method and route template, the path
// parameters and the canonical (RFC 8785) form of the body (ADR-0053 section 4).
// Two bodies that differ only in whitespace or member order have the same
// fingerprint. An empty body is allowed.
func Fingerprint(route string, pathParams []string, body []byte) ([sha256.Size]byte, error) {
	canonical := []byte{}
	if len(bytes.TrimSpace(body)) > 0 {
		v, err := jsonvalue.Parse(body)
		if err != nil {
			return [sha256.Size]byte{}, fmt.Errorf("parse the body: %w", err)
		}
		if canonical, err = jsonvalue.Canonical(v); err != nil {
			return [sha256.Size]byte{}, fmt.Errorf("canonicalize the body: %w", err)
		}
	}
	h := sha256.New()
	h.Write([]byte(route))
	h.Write([]byte{0})
	for _, p := range pathParams {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	h.Write(canonical)
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

// requesterOf is the exact requesting identity of a principal: the credential
// for keys and tokens, the principal itself for interactive sessions, so two
// keys of one user never share a record.
func requesterOf(p identity.Principal) string {
	if p.CredentialType != identity.CredentialSession && p.CredentialID != "" {
		return string(p.CredentialType) + ":" + p.CredentialID
	}
	return "PRINCIPAL:" + p.ID.String()
}

func validKey(s string) bool {
	if len(s) < minKeyLength || len(s) > maxKeyLength {
		return false
	}
	for i := range len(s) {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// recorder buffers the answer of the handler so it can be stored and then sent.
type recorder struct {
	header   http.Header
	status   int
	body     bytes.Buffer
	wrote    bool
	overflow bool
}

func (rec *recorder) Header() http.Header { return rec.header }

func (rec *recorder) WriteHeader(status int) {
	if !rec.wrote {
		rec.status = status
		rec.wrote = true
	}
}

func (rec *recorder) Write(p []byte) (int, error) {
	rec.WriteHeader(http.StatusOK)
	if rec.body.Len()+len(p) > maxStoredBody {
		rec.overflow = true
	}
	n, err := rec.body.Write(p)
	if err != nil {
		return n, fmt.Errorf("buffer the response: %w", err)
	}
	return n, nil
}

// response builds the stored form: only the headers a replay repeats, and no
// body when the response disclosed a secret.
func (rec *recorder) response(issuesSecret bool) *Response {
	header := http.Header{}
	for _, name := range []string{"Content-Type", "Location", "ETag"} {
		if v := rec.header.Get(name); v != "" {
			header.Set(name, v)
		}
	}
	resp := &Response{Status: rec.status, Header: header}
	if issuesSecret && rec.status >= 200 && rec.status < 300 {
		resp.SecretIssued = true
		return resp
	}
	resp.Body = bytes.Clone(rec.body.Bytes())
	return resp
}

func flush(w http.ResponseWriter, rec *recorder) {
	for name, values := range rec.header {
		for _, v := range values {
			w.Header().Add(name, v)
		}
	}
	w.WriteHeader(rec.status)
	_, _ = w.Write(rec.body.Bytes())
}
