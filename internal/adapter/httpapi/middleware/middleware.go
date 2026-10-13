// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package middleware holds the pieces of the request pipeline of the public API
// (ADR-0052 section 4): request id, panic recovery, body size limit,
// authentication, rate limits, the stability gate and authorization.
//
// Every piece has the signature func(http.Handler) http.Handler (API-5) and
// answers failures through package problem. The pipeline is assembled once, in
// package httpapi; the MCP server dispatches through the same assembly.
package middleware

import (
	"context"
	"net/http"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// Middleware wraps a handler.
type Middleware = func(http.Handler) http.Handler

// Chain applies middlewares so that the first one is the outermost.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type ctxKey int

const (
	requestIDKey ctxKey = iota
	principalKey
)

// RequestIDFrom returns the request id of ctx, or "" outside a request.
func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// PrincipalFrom returns the authenticated principal of ctx.
func PrincipalFrom(ctx context.Context) (identity.Principal, bool) {
	p, ok := ctx.Value(principalKey).(identity.Principal)
	return p, ok
}

// WithPrincipal returns a context carrying p. The authentication middleware
// calls it; tests use it to stand for an authenticated caller.
func WithPrincipal(ctx context.Context, p identity.Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// RequestIDHeader is the header that carries the request id.
const RequestIDHeader = "Orcon-Request-Id"

// RequestID gives every request an id (R-API-05): the one the client sent in
// Orcon-Request-Id when it is a well-formed UUID, otherwise a new one. The id
// is set on the response before anything else runs, so every answer, error
// documents included, carries it.
func RequestID(gen id.Generator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := ""
			if sent := r.Header.Get(RequestIDHeader); sent != "" {
				if parsed, err := id.Parse(sent); err == nil && !parsed.IsZero() {
					requestID = parsed.String()
				}
			}
			if requestID == "" {
				requestID = gen.New().String()
			}
			w.Header().Set(RequestIDHeader, requestID)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, requestID)))
		})
	}
}
