// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"errors"
	"log/slog"
)

// PrincipalKind says what kind of actor a [Principal] is.
type PrincipalKind string

// The kinds of principal (principal.kind, UPPER_SNAKE like the API).
const (
	// PrincipalUser is a person.
	PrincipalUser PrincipalKind = "USER"
	// PrincipalServiceAccount is a non-interactive identity.
	PrincipalServiceAccount PrincipalKind = "SERVICE_ACCOUNT"
	// PrincipalSystem is the installation acting on its own behalf.
	PrincipalSystem PrincipalKind = "SYSTEM"
)

// CredentialType says how a principal proved who it is (ADR-0056).
type CredentialType string

// The credential types.
const (
	// CredentialSession is an interactive session.
	CredentialSession CredentialType = "SESSION"
	// CredentialServiceAccountKey is a service account key (SAK).
	CredentialServiceAccountKey CredentialType = "SAK"
	// CredentialPersonalKey is a personal API key (PAK).
	CredentialPersonalKey CredentialType = "PAK"
	// CredentialOAuthToken is an OAuth access token (OAT).
	CredentialOAuthToken CredentialType = "OAT"
)

// Principal is who acts on a request, as the authentication chain resolved it.
// It carries no secret.
type Principal struct {
	// ID identifies the actor.
	ID PrincipalID
	// Kind says what the actor is.
	Kind PrincipalKind
	// UserID is the user of a user principal; zero for the other kinds.
	UserID UserID
	// OrgID is the organization context of the request. For key and token
	// principals it is fixed to the organization of the credential.
	OrgID OrgID
	// CredentialType says how the actor authenticated.
	CredentialType CredentialType
	// CredentialID identifies the session, key or token used, never its
	// secret. Idempotency records are scoped by it, so two keys of the same
	// person do not share records.
	CredentialID string
}

// IsZero reports whether p is the empty principal.
func (p Principal) IsZero() bool { return p.ID.IsZero() }

// Credentials is what a request presents to prove who it is. It holds secrets,
// so it never prints them.
type Credentials struct {
	// Bearer is the value of the Authorization bearer credential.
	Bearer string
	// Session is the value of the session cookie.
	Session string
	// CSRFToken is the anti-forgery token sent with a cookie session, in the
	// Orcon-Csrf-Token header.
	CSRFToken string
	// CSRFCookie is the value of the readable CSRF cookie; it must equal
	// CSRFToken and be the one derived from the session.
	CSRFCookie string
	// Origin is the Origin header of the request, empty when it has none.
	Origin string
	// UnsafeMethod is true for methods that change state; a cookie session
	// then needs the Origin of the installation and a valid CSRF token.
	UnsafeMethod bool
	// OrgHint is the organization the client asked for with Orcon-Org.
	OrgHint string
}

// Present reports whether the request presented any credential at all.
func (c Credentials) Present() bool { return c.Bearer != "" || c.Session != "" }

// String hides the secrets.
func (Credentials) String() string { return "identity.Credentials{[REDACTED]}" }

// GoString hides the secrets from %#v.
func (Credentials) GoString() string { return "identity.Credentials{[REDACTED]}" }

// LogValue hides the secrets from structured logs.
func (Credentials) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

// ErrUnauthenticated means no valid credential was presented. The edge turns it
// into a 401 answer.
var ErrUnauthenticated = errors.New("unauthenticated")

// ErrCSRFCheckFailed means a state-changing request authenticated by a cookie
// session did not carry the Origin of the installation and a matching CSRF token.
// The edge turns it into a 403 csrf-check-failed answer.
var ErrCSRFCheckFailed = errors.New("csrf check failed")

// Authenticator resolves the credentials of a request into a principal
// (ADR-0038 section 6, ADR-0056). A failure to authenticate is
// [ErrUnauthenticated]; any other error is an internal failure.
type Authenticator interface {
	Authenticate(ctx context.Context, creds Credentials) (Principal, error)
}

// RejectAll is the authenticator that authenticates nobody. The composition
// root wires it until the sign-in cut of SPEC-004 provides the real chain, so
// every protected route answers 401.
type RejectAll struct{}

// Authenticate implements [Authenticator].
func (RejectAll) Authenticate(context.Context, Credentials) (Principal, error) {
	return Principal{}, ErrUnauthenticated
}
