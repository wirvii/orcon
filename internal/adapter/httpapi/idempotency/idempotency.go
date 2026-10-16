// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package idempotency makes a POST safe to retry (ADR-0053 section 4): the first request
// with an Idempotency-Key runs, and a repeat with the same key and the same content gets the
// stored response with Idempotent-Replayed: true.
//
// The records live behind the [identity.IdempotencyStore] port. The change that a request
// performs claims its key in the transaction of the change (see
// internal/platform/idempotencyclaim), so a committed change always has its record. This
// package reads the record before the change, stores the response after it, and answers a
// repeat from the record: a completed record replays its response, an in-progress one answers
// 409 while its first request runs, and an in-progress one left by a crash is rebuilt from the
// resource it created (Amendment 8 section 8-2).
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
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/idempotencyclaim"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
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

// maxStoredBody bounds the response the middleware keeps in memory to store. A longer one
// is sent but not stored: its record stays in progress and a repeat rebuilds it from the
// resource.
const maxStoredBody = 1 << 20

// retryInProgress is the Retry-After of a request whose first run is still going.
const retryInProgress = time.Second

// staleAfter is the age after which an in-progress record is taken to be left by a crash:
// the write timeout of the server plus five seconds (Amendment 8 section 8-2).
const staleAfter = 35 * time.Second

// ErrNoRebuild means the operation cannot rebuild the response of a record that was never
// completed. The repeat then answers 409 idempotency-in-progress.
var ErrNoRebuild = errors.New("the response cannot be rebuilt")

// Rebuild returns the response that the resource named by rec implies, read in the organization
// of the request (authz.OrgScopeFrom): it is the answer a completed first request would have given. It is called only for an in-progress record
// whose change committed (it names its resource) and that is older than staleAfter.
type Rebuild func(ctx context.Context, rec identity.IdempotencyRecord) (identity.IdempotencyResponse, error)

// Config configures the middleware of one operation.
type Config struct {
	// Store keeps the records.
	Store identity.IdempotencyStore
	// Clock stamps the records and decides when an in-progress record is stale.
	Clock clock.Clock
	// IDs creates the identity of the record a claiming transaction writes.
	IDs id.Generator
	// Logger receives store failures.
	Logger *slog.Logger
	// Required makes a missing key a 400 validation-failed.
	Required bool
	// IssuesSecret is true when a 2xx response of the operation discloses a secret: the record
	// keeps no body, and a repeat is refused with 409 secret-already-issued.
	IssuesSecret bool
	// Route is the route template, "POST /api/v1/flows/{flowId}/executions".
	Route string
	// Rebuild answers the repeat of an in-progress record left by a crash. Nil means such a
	// repeat answers 409 idempotency-in-progress.
	Rebuild Rebuild
}

// Middleware returns the idempotency step of the pipeline for one operation. It must run after
// authentication and authorization, because the record is kept in the organization that the
// authorization decided.
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
	scope, ok := authz.OrgScopeFrom(r.Context())
	if !ok {
		cfg.Logger.ErrorContext(r.Context(), "idempotency: the request has no authorized organization",
			"request_id", middleware.RequestIDFrom(r.Context()))
		problem.Write(w, r, problem.InternalError)
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
	key := identity.IdempotencyKey{Requester: requesterOf(principal), Value: value}
	now := cfg.Clock.Now()
	rec, found, err := cfg.Store.LookupIdempotency(r.Context(), scope, key, now)
	if err != nil {
		storeFailed(cfg, w, r, "lookup", err)
		return
	}
	if found {
		answerExisting(cfg, w, r, scope, key, rec, fingerprint, now)
		return
	}
	run(cfg, next, w, r, scope, key, fingerprint, now)
}

// run executes the operation with its claim in the context, stores the response of a success
// and sends it.
func run(cfg *Config, next http.Handler, w http.ResponseWriter, r *http.Request, scope orgscope.Org, key identity.IdempotencyKey, fingerprint [sha256.Size]byte, now time.Time) {
	claim := idempotencyclaim.Claim{ID: cfg.IDs.New(), Requester: key.Requester, Value: key.Value, Fingerprint: fingerprint}
	rec := &recorder{header: http.Header{}, status: http.StatusOK}
	// The bookkeeping outlives a client that went away.
	ctx := context.WithoutCancel(r.Context())
	next.ServeHTTP(rec, r.WithContext(idempotencyclaim.With(r.Context(), claim)))

	if rec.status < http.StatusOK || rec.status >= http.StatusMultipleChoices {
		flush(w, rec)
		return
	}
	if rec.overflow {
		cfg.Logger.WarnContext(ctx, "idempotency: the response is too large to store; a repeat rebuilds it",
			"request_id", middleware.RequestIDFrom(ctx))
		flush(w, rec)
		return
	}
	resp := rec.response()
	if cfg.IssuesSecret {
		// The secret is shown once: the record keeps none of it, and a repeat is refused.
		resp.SecretIssued = true
		resp.Body = nil
	}
	c := identity.IdempotencyCompletion{ID: claim.ID, Fingerprint: fingerprint, Response: resp}
	if err := cfg.Store.CompleteIdempotency(ctx, scope, key, c, now); err != nil {
		cfg.Logger.ErrorContext(ctx, "idempotency: complete failed", "request_id", middleware.RequestIDFrom(ctx), "error", err)
	}
	flush(w, rec)
}

// answerExisting answers a request whose key already has a record.
func answerExisting(cfg *Config, w http.ResponseWriter, r *http.Request, scope orgscope.Org, key identity.IdempotencyKey, rec identity.IdempotencyRecord, fingerprint [sha256.Size]byte, now time.Time) {
	switch {
	case rec.Fingerprint != fingerprint:
		problem.Write(w, r, problem.IdempotencyKeyReused,
			problem.Detail("The Idempotency-Key was used with a different request."))
	case rec.Completed && rec.Response.SecretIssued:
		opts := []problem.Option{problem.Detail("The secret of this request was shown once and is not stored; read the resource instead.")}
		if rec.Response.Location != "" {
			opts = append(opts, problem.Header("Location", rec.Response.Location))
		}
		problem.Write(w, r, problem.SecretAlreadyIssued, opts...)
	case rec.Completed:
		replay(w, r, rec.Response)
	case now.Sub(rec.CreatedAt) < staleAfter || cfg.Rebuild == nil || rec.ResourceRef == "":
		problem.Write(w, r, problem.IdempotencyInProgress, problem.RetryAfter(retryInProgress),
			problem.Detail("A request with this Idempotency-Key is still running."))
	default:
		resp, err := cfg.Rebuild(r.Context(), rec)
		if errors.Is(err, ErrNoRebuild) {
			problem.Write(w, r, problem.IdempotencyInProgress, problem.RetryAfter(retryInProgress),
				problem.Detail("A request with this Idempotency-Key is still running."))
			return
		}
		if err != nil {
			storeFailed(cfg, w, r, "rebuild", err)
			return
		}
		c := identity.IdempotencyCompletion{ID: cfg.IDs.New(), Fingerprint: rec.Fingerprint, Response: resp}
		if err := cfg.Store.CompleteIdempotency(context.WithoutCancel(r.Context()), scope, key, c, now); err != nil {
			cfg.Logger.ErrorContext(r.Context(), "idempotency: complete after rebuild failed",
				"request_id", middleware.RequestIDFrom(r.Context()), "error", err)
		}
		replay(w, r, resp)
	}
}

// replay sends a stored response with Idempotent-Replayed: true. A HEAD request gets no body.
func replay(w http.ResponseWriter, r *http.Request, resp identity.IdempotencyResponse) {
	if resp.ContentType != "" {
		w.Header().Set("Content-Type", resp.ContentType)
	}
	if resp.Location != "" {
		w.Header().Set("Location", resp.Location)
	}
	if resp.ETag != "" {
		w.Header().Set("ETag", resp.ETag)
	}
	w.Header().Set(ReplayedHeader, "true")
	w.WriteHeader(resp.Status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(resp.Body)
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

// Fingerprint is the SHA-256 of the method and route template, the path parameters and the
// canonical (RFC 8785) form of the body (ADR-0053 section 4). Two bodies that differ only in
// whitespace or member order have the same fingerprint. An empty body is allowed.
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

// requesterOf is the exact requesting identity of a principal: the credential for keys and
// tokens, the principal itself for interactive sessions, so two keys of one user never share
// a record.
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

// response builds the stored form: the status, the headers a replay repeats and the body.
func (rec *recorder) response() identity.IdempotencyResponse {
	return identity.IdempotencyResponse{
		Status:      rec.status,
		ContentType: rec.header.Get("Content-Type"),
		Location:    rec.header.Get("Location"),
		ETag:        rec.header.Get("ETag"),
		Body:        bytes.Clone(rec.body.Bytes()),
	}
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
