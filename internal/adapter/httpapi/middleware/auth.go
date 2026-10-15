// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
)

// AuthConfig names the places a request carries its credentials.
type AuthConfig struct {
	// SessionCookie is the name of the session cookie (SPEC-016 R-SH-API-04).
	SessionCookie string
	// CSRFCookie is the name of the readable cookie that holds the CSRF value of the session.
	CSRFCookie string
	// CSRFHeader is the header that carries the anti-forgery token of a cookie session.
	CSRFHeader string
	// OrgHeader is the header with which a client chooses its organization.
	OrgHeader string
}

// DefaultAuthConfig returns the names of the contract: the cookies are host-only
// (the __Host- prefix) and the CSRF header is Orcon-Csrf-Token.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		SessionCookie: "__Host-orcon_session",
		CSRFCookie:    "__Host-orcon_csrf",
		CSRFHeader:    "Orcon-Csrf-Token",
		OrgHeader:     "Orcon-Org",
	}
}

type credentialsKey struct{}

// CarryCredentials stores the credentials the request presents in its context, for
// every operation, public ones included, so a handler can read the Origin and the
// session of the caller (SPEC-016 R-SH-SES-03). It authenticates nobody.
func CarryCredentials(cfg AuthConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), credentialsKey{}, credentialsOf(r, cfg))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithCredentials returns ctx carrying c, as [CarryCredentials] stores them. It lets
// a caller that has no request at hand (a test, a handler run directly) present the
// same credentials.
func WithCredentials(ctx context.Context, c identity.Credentials) context.Context {
	return context.WithValue(ctx, credentialsKey{}, c)
}

// CredentialsFrom returns the credentials the request presented, as
// [CarryCredentials] stored them.
func CredentialsFrom(ctx context.Context) (identity.Credentials, bool) {
	c, ok := ctx.Value(credentialsKey{}).(identity.Credentials)
	return c, ok
}

// Authenticate resolves the credentials of the request into a principal and
// stores it in the context (pipeline step 3). A request with no credential, or
// one the authenticator rejects, gets 401 unauthenticated; a cookie session that
// changes state without the Origin and CSRF token of the installation gets 403
// csrf-check-failed; an authenticator failure gets 500 and is logged once.
func Authenticate(a identity.Authenticator, logger *slog.Logger, cfg AuthConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			creds := credentialsOf(r, cfg)
			if !creds.Present() {
				unauthenticated(w, r)
				return
			}
			principal, err := a.Authenticate(r.Context(), creds)
			switch {
			case errors.Is(err, identity.ErrUnauthenticated):
				unauthenticated(w, r)
				return
			case errors.Is(err, identity.ErrCSRFCheckFailed):
				problem.Write(w, r, problem.CSRFCheckFailed)
				return
			case err != nil:
				logger.ErrorContext(r.Context(), "authentication failed",
					"request_id", RequestIDFrom(r.Context()), "error", err)
				problem.Write(w, r, problem.InternalError)
				return
			case principal.IsZero():
				logger.ErrorContext(r.Context(), "authenticator returned an empty principal",
					"request_id", RequestIDFrom(r.Context()))
				problem.Write(w, r, problem.InternalError)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
		})
	}
}

func unauthenticated(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	problem.Write(w, r, problem.Unauthenticated)
}

func credentialsOf(r *http.Request, cfg AuthConfig) identity.Credentials {
	creds := identity.Credentials{
		CSRFToken:    r.Header.Get(cfg.CSRFHeader),
		Origin:       r.Header.Get("Origin"),
		UnsafeMethod: isUnsafe(r.Method),
		OrgHint:      r.Header.Get(cfg.OrgHeader),
	}
	if scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " "); ok && strings.EqualFold(scheme, "Bearer") {
		creds.Bearer = strings.TrimSpace(token)
	}
	if c, err := r.Cookie(cfg.SessionCookie); err == nil {
		creds.Session = c.Value
	}
	if c, err := r.Cookie(cfg.CSRFCookie); err == nil {
		creds.CSRFCookie = c.Value
	}
	return creds
}

func isUnsafe(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

// ResourceOf derives the resource a request addresses from the path
// parameters of its route.
type ResourceOf func(r *http.Request) authz.ResourceRef

// Authorize asks the authorizer whether the principal may do what the
// operation requires (pipeline step 6). The decision maps to the visibility
// rules of ADR-0041 section 6: forbidden is 403, hidden is 404. A requirement
// that needs no decision (authenticated or anonymous) passes through.
func Authorize(a authz.Authorizer, logger *slog.Logger, need authz.Requirement, resource ResourceOf) Middleware {
	return func(next http.Handler) http.Handler {
		if !need.NeedsDecision() {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFrom(r.Context())
			if !ok {
				unauthenticated(w, r)
				return
			}
			decision, scope, err := a.Authorize(r.Context(), authz.Request{
				Principal:   principal,
				Requirement: need,
				Resource:    resource(r),
			})
			if err != nil {
				logger.ErrorContext(r.Context(), "authorization failed",
					"request_id", RequestIDFrom(r.Context()), "error", err)
				problem.Write(w, r, problem.InternalError)
				return
			}
			switch decision {
			case authz.DecisionAllow:
				if !scope.UUID().IsZero() {
					r = r.WithContext(authz.WithOrgScope(r.Context(), scope))
				}
				next.ServeHTTP(w, r)
			case authz.DecisionHidden:
				problem.Write(w, r, problem.NotFound)
			case authz.DecisionForbidden:
				problem.Write(w, r, problem.Forbidden)
			default:
				// An unknown decision denies: deny by default (ADR-0041 section 6).
				problem.Write(w, r, problem.Forbidden)
			}
		})
	}
}
