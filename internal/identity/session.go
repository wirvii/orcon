// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// SessionPolicy bounds a login session (SPEC-004 R-ID-06): the maximum age, whatever
// the activity, and the idle timeout, which activity extends.
type SessionPolicy struct {
	MaxAge      time.Duration
	IdleTimeout time.Duration
}

// DefaultSessionPolicy is the default of R-ID-06: eight hours idle, seven days at most.
func DefaultSessionPolicy() SessionPolicy {
	return SessionPolicy{MaxAge: 7 * 24 * time.Hour, IdleTimeout: 8 * time.Hour}
}

// ThrottlePolicy bounds the failed sign-in attempts of one identifier (SPEC-004
// R-AUTH-02): the first FreeAttempts failures are free; each later one doubles the
// wait from BaseDelay up to MaxDelay. Nothing is locked for good.
type ThrottlePolicy struct {
	FreeAttempts int
	BaseDelay    time.Duration
	MaxDelay     time.Duration
}

// DefaultThrottlePolicy is five free attempts, then 30 s doubling up to 15 min.
func DefaultThrottlePolicy() ThrottlePolicy {
	return ThrottlePolicy{FreeAttempts: 5, BaseDelay: 30 * time.Second, MaxDelay: 15 * time.Minute}
}

// Delay returns how long an identifier with that many failures waits.
func (p ThrottlePolicy) Delay(failures int) time.Duration {
	steps := min(failures-p.FreeAttempts, 20)
	if steps < 0 {
		return 0
	}
	d := p.BaseDelay * time.Duration(math.Pow(2, float64(steps)))
	if d <= 0 || d > p.MaxDelay {
		return p.MaxDelay
	}
	return d
}

// touchInterval is how old the last activity may be before a request moves the idle
// deadline; it keeps a busy session from writing on every request.
const touchInterval = time.Minute

// PasswordHasher hashes and checks passwords (ADR-0039 section 1). Verify takes the
// same time for a wrong and for a right password; platform/password implements it.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(encoded, password string) (bool, error)
	NeedsRehash(encoded string) bool
}

// Identifier turns a normalized address into the 16-byte key that counts failed
// sign-ins (SPEC-004 R-AUTH-02). It is a keyed hash, so the address never reaches the
// throttle table.
type Identifier func(emailNormalized string) []byte

// IdentifierSize is the length of an identifier: 128 bits of a keyed hash.
const IdentifierSize = 16

// HMACIdentifier returns the identifier that is the HMAC-SHA-256 of the address under
// key, truncated to IdentifierSize bytes. The key needs at least 32 bytes; the
// composition root gives it the instance key.
func HMACIdentifier(key []byte) (Identifier, error) {
	if len(key) < 32 {
		return nil, fmt.Errorf("%w: the identifier key needs at least 32 bytes", ErrInvalidInput)
	}
	k := slices.Clone(key)
	return func(email string) []byte {
		m := hmac.New(sha256.New, k)
		m.Write([]byte(email))
		return m.Sum(nil)[:IdentifierSize]
	}, nil
}

// NormalizeEmail returns the form in which an address is stored and compared.
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// csrfContext separates the CSRF derivation from every other use of a session token.
const csrfContext = "orcon csrf v1\x00"

// CSRFTokenFor returns the anti-forgery value bound to a session token. The readable
// CSRF cookie holds it and the Orcon-Csrf-Token header must repeat it, so it works for
// that session only (SPEC-004 R-ID-06, SPEC-016 R-SH-SES-01).
func CSRFTokenFor(sessionToken string) string {
	sum := sha256.Sum256([]byte(csrfContext + sessionToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// tokenHash is what the database keeps of a session token.
func tokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// SignInReason says why a sign-in was refused. The HTTP answer is the same for the
// first four (SPEC-004 R-AUTH-02); the reason goes to the audit log, never to the client.
type SignInReason string

// The reasons of a refused sign-in.
const (
	ReasonUnknownAccount SignInReason = "UNKNOWN_ACCOUNT"
	ReasonWrongPassword  SignInReason = "WRONG_PASSWORD"
	ReasonDisabled       SignInReason = "DISABLED"
	ReasonNoPassword     SignInReason = "NO_PASSWORD"
	ReasonThrottled      SignInReason = "THROTTLED"
)

// ErrSignInError is what every refused sign-in satisfies with errors.Is.
var ErrSignInError = errors.New("sign-in rejected")

// SignInError is a refused sign-in. RetryAfter is set only for [ReasonThrottled].
type SignInError struct {
	Reason     SignInReason
	RetryAfter time.Duration
}

// Error implements error. It names no reason, so no log line can reveal the account.
func (e *SignInError) Error() string { return ErrSignInError.Error() }

// Unwrap makes errors.Is(err, ErrSignInError) true.
func (e *SignInError) Unwrap() error { return ErrSignInError }

// SessionDeps are the ports the session service needs.
type SessionDeps struct {
	Users    UserStore
	Sessions SessionStore
	Throttle LoginThrottleStore
	Hasher   PasswordHasher
	Identify Identifier
	Clock    clock.Clock
	IDs      id.Generator
	// Audit receives the entries of the attempts that have no transaction of their own.
	Audit AuditLog
	// Random supplies the session tokens; nil means crypto/rand.
	Random io.Reader
}

// SessionConfig is the configuration of the session service.
type SessionConfig struct {
	// PublicOrigin is the origin of the installation (ORCON_PUBLIC_URL): the only
	// Origin a state-changing cookie request may carry.
	PublicOrigin string
	// Session and Attempts take their defaults when zero.
	Session  SessionPolicy
	Attempts ThrottlePolicy
}

// Sessions signs users in and out, and resolves the session cookie of a request.
type Sessions struct {
	users    UserStore
	sessions SessionStore
	throttle LoginThrottleStore
	hasher   PasswordHasher
	identify Identifier
	clock    clock.Clock
	ids      id.Generator
	audit    AuditLog
	random   io.Reader
	origin   string
	policy   SessionPolicy
	attempts ThrottlePolicy
	// dummy is a hash of a password no account has. Sign-in checks it when the
	// account does not exist, so an unknown address costs as much as a known one.
	dummy string
}

var _ Authenticator = (*Sessions)(nil)

// NewSessions returns the session service. It hashes a dummy password once, so it
// fails when the hasher does.
func NewSessions(deps SessionDeps, cfg SessionConfig) (*Sessions, error) {
	if cfg.Session == (SessionPolicy{}) {
		cfg.Session = DefaultSessionPolicy()
	}
	if cfg.Attempts == (ThrottlePolicy{}) {
		cfg.Attempts = DefaultThrottlePolicy()
	}
	random := deps.Random
	if random == nil {
		random = rand.Reader
	}
	if deps.Audit == nil {
		return nil, errors.New("sessions: the audit log is required (SPEC-004 R-AUD-02)")
	}
	dummy, err := deps.Hasher.Hash("no account has this password")
	if err != nil {
		return nil, fmt.Errorf("sessions: dummy hash: %w", err)
	}
	return &Sessions{
		users: deps.Users, sessions: deps.Sessions, throttle: deps.Throttle, hasher: deps.Hasher,
		identify: deps.Identify, clock: deps.Clock, ids: deps.IDs, audit: deps.Audit, random: random,
		origin: cfg.PublicOrigin, policy: cfg.Session, attempts: cfg.Attempts, dummy: dummy,
	}, nil
}

// entry builds the audit entry of an action of the session service, stamped with
// the clock at the precision the database keeps.
func (s *Sessions) entry(actor audit.Actor, action string, outcome audit.Outcome, details jsonvalue.Value) (audit.Entry, error) {
	e, err := audit.NewEntry(audit.EntryParams{
		ID: s.ids.New(), Chain: audit.InstanceChain(), OccurredAt: s.instant(), Actor: actor,
		Action: action, Outcome: outcome, Details: details,
	})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("audit %s: %w", action, err)
	}
	return e, nil
}

// userActor is the actor of a user principal.
func userActor(p PrincipalID) audit.Actor {
	return audit.Actor{PrincipalID: id.UUID(p), Kind: audit.ActorUser}
}

// failureDetails are the fields of a failed or refused attempt: the reason and the
// identifier, already an HMAC of the address (R-AUD-08), never the address itself.
func failureDetails(reason SignInReason, identifier []byte) (jsonvalue.Value, error) {
	return jsonvalue.Object(
		jsonvalue.Member{Key: "reason", Value: jsonvalue.String(string(reason))},
		jsonvalue.Member{Key: "identifier", Value: jsonvalue.String(hex.EncodeToString(identifier))},
	)
}

// SignIn is the input of [Sessions.SignIn].
type SignIn struct {
	Email    string
	Password string
	// PriorToken is the token of the session the request already carries. A successful
	// sign-in revokes it, so every sign-in starts a new session (SPEC-016 R-SH-SES-01).
	PriorToken string
}

// SignedIn is a session started by [Sessions.SignIn]. Token and CSRFToken go to
// cookies only; the session row keeps the hash of the token.
type SignedIn struct {
	Token     string
	CSRFToken string
	Session   SessionRecord
	User      UserRecord
}

// SignIn checks the address and the password and starts a session. It refuses an
// address whose identifier is throttled before it looks the account up. Every other
// refusal is a [SignInError] with its reason; the caller answers all of them alike
// except the throttled one.
func (s *Sessions) SignIn(ctx context.Context, in SignIn) (SignedIn, error) {
	email := NormalizeEmail(in.Email)
	key := s.identify(email)
	now := s.instant()

	state, err := s.throttle.GetLoginThrottle(ctx, key)
	known := err == nil
	switch {
	case known && now.Before(state.LockedUntil):
		if err := s.auditAttempt(ctx, audit.Actor{}, audit.OutcomeDenied, ReasonThrottled, key); err != nil {
			return SignedIn{}, err
		}
		return SignedIn{}, &SignInError{Reason: ReasonThrottled, RetryAfter: state.LockedUntil.Sub(now)}
	case err != nil && !errors.Is(err, ErrNotFound):
		return SignedIn{}, fmt.Errorf("sign in: %w", err)
	}

	creds, err := s.users.GetUserCredentialsByEmail(ctx, email)
	found := err == nil
	if err != nil && !errors.Is(err, ErrNotFound) {
		return SignedIn{}, fmt.Errorf("sign in: %w", err)
	}
	hash := s.dummy
	if found && creds.PasswordHash != "" {
		hash = creds.PasswordHash
	}
	// The check runs for every account, known or not, so the time does not reveal which.
	ok, err := s.hasher.Verify(hash, in.Password)
	if err != nil {
		return SignedIn{}, fmt.Errorf("sign in: check password: %w", err)
	}

	var reason SignInReason
	switch {
	case !found:
		reason = ReasonUnknownAccount
	case creds.User.Status == UserDisabled:
		reason = ReasonDisabled
	case creds.PasswordHash == "":
		reason = ReasonNoPassword
	case !ok:
		reason = ReasonWrongPassword
	}
	if reason != "" {
		if err := s.recordFailure(ctx, key, now); err != nil {
			return SignedIn{}, err
		}
		var actor audit.Actor
		if found {
			actor = userActor(creds.User.PrincipalID)
		}
		if err := s.auditAttempt(ctx, actor, audit.OutcomeFailure, reason, key); err != nil {
			return SignedIn{}, err
		}
		return SignedIn{}, &SignInError{Reason: reason}
	}
	return s.start(ctx, in, key, now, creds)
}

// auditAttempt appends the login.failed entry of a refused or failed sign-in in its own
// transaction (R-AUD-02: a denied action is audited in its own transaction).
func (s *Sessions) auditAttempt(ctx context.Context, actor audit.Actor, outcome audit.Outcome, reason SignInReason, identifier []byte) error {
	details, err := failureDetails(reason, identifier)
	if err != nil {
		return fmt.Errorf("sign in: audit details: %w", err)
	}
	e, err := s.entry(actor, "login.failed", outcome, details)
	if err != nil {
		return fmt.Errorf("sign in: %w", err)
	}
	if err := s.audit.AppendInstanceEntry(ctx, e); err != nil {
		return fmt.Errorf("sign in: audit: %w", err)
	}
	return nil
}

// start opens the session of a user whose password checked out.
func (s *Sessions) start(ctx context.Context, in SignIn, key []byte, now time.Time, creds UserCredentials) (SignedIn, error) {
	if err := s.throttle.ClearLoginThrottle(ctx, key); err != nil {
		return SignedIn{}, fmt.Errorf("sign in: %w", err)
	}
	if s.hasher.NeedsRehash(creds.PasswordHash) {
		fresh, err := s.hasher.Hash(in.Password)
		if err != nil {
			return SignedIn{}, fmt.Errorf("sign in: rehash: %w", err)
		}
		if err := s.users.SetPasswordHash(ctx, creds.User.ID, fresh, now); err != nil {
			return SignedIn{}, fmt.Errorf("sign in: store rehash: %w", err)
		}
	}
	actor := userActor(creds.User.PrincipalID)
	if in.PriorToken != "" {
		if err := s.revokePrior(ctx, in.PriorToken, now, actor); err != nil {
			return SignedIn{}, err
		}
	}
	token, err := s.newToken()
	if err != nil {
		return SignedIn{}, err
	}
	rec := SessionRecord{
		ID:             SessionID(s.ids.New()),
		UserID:         creds.User.ID,
		TokenHash:      tokenHash(token),
		AssuranceLevel: AssuranceAAL1,
		CreatedAt:      now,
		ExpiresAt:      now.Add(s.policy.MaxAge),
		IdleExpiresAt:  idleDeadline(now, now.Add(s.policy.MaxAge), s.policy.IdleTimeout),
		LastSeenAt:     now,
	}
	e, err := s.entry(actor, "login.succeeded", audit.OutcomeSuccess, jsonvalue.Value{})
	if err != nil {
		return SignedIn{}, fmt.Errorf("sign in: %w", err)
	}
	if err := s.sessions.CreateSession(ctx, rec, e); err != nil {
		return SignedIn{}, fmt.Errorf("sign in: %w", err)
	}
	return SignedIn{Token: token, CSRFToken: CSRFTokenFor(token), Session: rec, User: creds.User}, nil
}

// recordFailure counts one failed attempt of the identifier in the store, and past the
// free attempts locks it for the delay of the count that the store returned.
func (s *Sessions) recordFailure(ctx context.Context, key []byte, now time.Time) error {
	failures, err := s.throttle.IncrementLoginThrottle(ctx, key, now)
	if err != nil {
		return fmt.Errorf("sign in: record failure: %w", err)
	}
	if failures >= s.attempts.FreeAttempts {
		if err := s.throttle.LockLoginThrottle(ctx, key, now.Add(s.attempts.Delay(failures)), now); err != nil {
			return fmt.Errorf("sign in: lock: %w", err)
		}
	}
	return nil
}

// revokePrior revokes the session the request already carried, when it is still live.
func (s *Sessions) revokePrior(ctx context.Context, token string, now time.Time, actor audit.Actor) error {
	view, err := s.sessions.GetSessionByTokenHash(ctx, tokenHash(token))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("sign in: previous session: %w", err)
	}
	if !view.Session.RevokedAt.IsZero() {
		return nil
	}
	e, err := s.entry(actor, "session.revoked", audit.OutcomeSuccess, jsonvalue.Value{})
	if err != nil {
		return fmt.Errorf("sign in: %w", err)
	}
	if err := s.sessions.MarkSessionRevoked(ctx, view.Session.UserID, view.Session.ID, now, e); err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("sign in: revoke previous session: %w", err)
	}
	return nil
}

// Authenticate resolves the session cookie of a request into a user principal
// (SPEC-004 R-ID-06). A state-changing request must also carry the Origin of the
// installation and a CSRF token that matches the session; otherwise it fails with
// [ErrCSRFCheckFailed]. A request without a live session fails with [ErrUnauthenticated].
func (s *Sessions) Authenticate(ctx context.Context, creds Credentials) (Principal, error) {
	if creds.Session == "" {
		return Principal{}, ErrUnauthenticated
	}
	now := s.instant()
	view, err := s.sessions.GetSessionByTokenHash(ctx, tokenHash(creds.Session))
	if errors.Is(err, ErrNotFound) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, fmt.Errorf("authenticate: %w", err)
	}
	sess := view.Session
	if !sess.RevokedAt.IsZero() || !now.Before(sess.ExpiresAt) || !now.Before(sess.IdleExpiresAt) || view.UserStatus != UserActive {
		return Principal{}, ErrUnauthenticated
	}
	if creds.UnsafeMethod {
		want := CSRFTokenFor(creds.Session)
		if s.origin == "" || creds.Origin != s.origin || creds.CSRFToken == "" ||
			creds.CSRFToken != creds.CSRFCookie || creds.CSRFToken != want {
			return Principal{}, ErrCSRFCheckFailed
		}
	}
	if now.Sub(sess.LastSeenAt) >= touchInterval {
		err := s.sessions.UpdateSessionActivity(ctx, sess.ID, now, idleDeadline(now, sess.ExpiresAt, s.policy.IdleTimeout))
		if errors.Is(err, ErrNotFound) {
			return Principal{}, ErrUnauthenticated
		}
		if err != nil {
			return Principal{}, fmt.Errorf("authenticate: %w", err)
		}
	}
	return Principal{
		ID: view.PrincipalID, Kind: PrincipalUser, UserID: sess.UserID, OrgID: orgHint(creds.OrgHint),
		CredentialType: CredentialSession, CredentialID: sess.ID.String(),
	}, nil
}

// orgHint reads the organization a client named with Orcon-Org (ADR-0038 section 6). The
// organization is only a context: the authorization evaluator refuses it unless the user
// holds an active membership there, and a value that is not an identifier leaves the
// context empty, so every protected operation is refused.
func orgHint(hint string) OrgID {
	if hint == "" {
		return OrgID{}
	}
	u, err := id.Parse(hint)
	if err != nil {
		return OrgID{}
	}
	return OrgID(u)
}

// CurrentUser is the signed-in user and the session a request carries.
type CurrentUser struct {
	User    UserRecord
	Session SessionRecord
}

// Current returns the user and the session of an authenticated principal.
func (s *Sessions) Current(ctx context.Context, principal Principal) (CurrentUser, error) {
	sid, err := id.Parse(principal.CredentialID)
	if err != nil {
		return CurrentUser{}, fmt.Errorf("current session: %w", ErrNotFound)
	}
	sess, err := s.sessions.GetSession(ctx, principal.UserID, SessionID(sid))
	if err != nil {
		return CurrentUser{}, fmt.Errorf("current session: %w", err)
	}
	user, err := s.users.GetUser(ctx, principal.UserID)
	if err != nil {
		return CurrentUser{}, fmt.Errorf("current user: %w", err)
	}
	return CurrentUser{User: user, Session: sess}, nil
}

// MaxListedSessions bounds how many live sessions of one user a listing reads.
const MaxListedSessions = 1000

// ListMine returns the live sessions of the user who acts, newest first, at most
// [MaxListedSessions] (SPEC-004 R-ID-08). A revoked or expired session is not listed.
func (s *Sessions) ListMine(ctx context.Context, actor Principal) ([]SessionRecord, error) {
	out, err := s.sessions.ListLiveSessions(ctx, actor.UserID, s.instant(), MaxListedSessions)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return out, nil
}

// Revoke ends one live session of the user who acts (SPEC-016 R-SH-SES-04, SPEC-004 R-ID-08).
// The session the request carries ends with the logout entry, any other session of the same user
// with session.revoked; both record the principal that asked for it (Annex B, auth). A session
// of another user, an unknown one and one that is already revoked are [ErrNotFound]. The audit
// entry is stored in the transaction of the revocation.
func (s *Sessions) Revoke(ctx context.Context, actor Principal, session SessionID) error {
	action := "session.revoked"
	if session.String() == actor.CredentialID {
		action = "logout"
	}
	e, err := s.entry(userActor(actor.ID), action, audit.OutcomeSuccess, jsonvalue.Value{})
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if err := s.sessions.MarkSessionRevoked(ctx, actor.UserID, session, s.instant(), e); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// MaxDisplayNameLength is the longest display name, in characters (the column's check).
const MaxDisplayNameLength = 200

// ValidateDisplayName checks a display name: not blank and at most [MaxDisplayNameLength]
// characters. It returns the name without the spaces around it.
func ValidateDisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n == 0 || n > MaxDisplayNameLength {
		return "", fmt.Errorf("%w: the display name must have between 1 and %d characters", ErrInvalidInput, MaxDisplayNameLength)
	}
	return name, nil
}

// Rename changes the display name of the user who acts, only when the user still has the update
// instant the caller read (the revision behind the ETag). A user changed since it was read is
// [ErrConflict]. The account.renamed entry is stored in the same transaction.
func (s *Sessions) Rename(ctx context.Context, actor Principal, displayName string, expected time.Time) (UserRecord, error) {
	name, err := ValidateDisplayName(displayName)
	if err != nil {
		return UserRecord{}, err
	}
	details, err := jsonvalue.Object(jsonvalue.Member{Key: "userId", Value: jsonvalue.String(actor.UserID.String())})
	if err != nil {
		return UserRecord{}, fmt.Errorf("rename: %w", err)
	}
	now := s.instant()
	e, err := audit.NewEntry(audit.EntryParams{
		ID: s.ids.New(), Chain: audit.InstanceChain(), OccurredAt: now, Actor: userActor(actor.ID),
		Action: "account.renamed", Outcome: audit.OutcomeSuccess, Details: details,
	})
	if err != nil {
		return UserRecord{}, fmt.Errorf("rename: audit: %w", err)
	}
	if err := s.users.ChangeDisplayName(ctx, actor.UserID, name, expected, now, e); err != nil {
		return UserRecord{}, fmt.Errorf("rename: %w", err)
	}
	user, err := s.users.GetUser(ctx, actor.UserID)
	if err != nil {
		return UserRecord{}, fmt.Errorf("rename: %w", err)
	}
	return user, nil
}

// instant reads the clock at the precision the database keeps (microseconds), so the
// instants the service computes and the ones it stores are the same values.
func (s *Sessions) instant() time.Time { return clock.Micros(s.clock.Now()) }

// newToken returns a fresh opaque session token of 256 random bits.
func (s *Sessions) newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return "", fmt.Errorf("session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// idleDeadline is the idle timeout counted from now, never past the maximum age.
func idleDeadline(now, maxAge time.Time, idle time.Duration) time.Time {
	deadline := now.Add(idle)
	if deadline.After(maxAge) {
		return maxAge
	}
	return deadline
}
