// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
)

// Recover turns a panic in a handler into a 500 internal-error problem. The
// panic value and the stack are logged once, with the request id; nothing of
// them reaches the client (API-4.1).
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					answerPanic(w, r, logger, rec)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// answerPanic logs a recovered panic and answers 500.
func answerPanic(w http.ResponseWriter, r *http.Request, logger *slog.Logger, rec any) {
	if rec == http.ErrAbortHandler { //nolint:errorlint // http.ErrAbortHandler is compared by identity as net/http documents
		panic(rec)
	}
	logger.ErrorContext(r.Context(), "panic in handler",
		"request_id", RequestIDFrom(r.Context()),
		"method", r.Method,
		"path", r.URL.Path,
		"panic", rec,
		"stack", string(debug.Stack()),
	)
	problem.Write(w, r, problem.InternalError)
}

// BodyLimit bounds request bodies at limit bytes (API-5.2). A request that
// announces a longer body is refused at once with 413 payload-too-large; a
// body that turns out longer fails its read with *http.MaxBytesError, which the
// error handler of the router maps to the same problem.
func BodyLimit(limit int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				problem.Write(w, r, problem.PayloadTooLarge)
				return
			}
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ExperimentalHeader is the header an experimental operation requires.
const ExperimentalHeader = "Orcon-Experimental"

// ExperimentalGate refuses a request to an experimental operation unless
// Orcon-Experimental names the feature (VER-1). The header holds a
// comma-separated list of features. Callers install it only on experimental
// operations.
func ExperimentalGate(feature string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, value := range r.Header.Values(ExperimentalHeader) {
				for _, named := range strings.Split(value, ",") {
					if strings.EqualFold(strings.TrimSpace(named), feature) {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			problem.Write(w, r, problem.ExperimentalNotEnabled,
				problem.Detail("This operation is experimental. Send "+ExperimentalHeader+": "+feature+" to use it."))
		})
	}
}
