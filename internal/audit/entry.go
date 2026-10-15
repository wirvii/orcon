// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package audit records who did what, in an append-only chain that a later
// verification can check (SPEC-004 section 9, ADR-0048).
//
// An action inserts an [Entry] as a pending row in its own transaction, so the
// action and its entry commit together or not at all (R-AUD-02). A [Sealer] turns
// pending entries into sealed events: each event gets a sequence number and a hash
// that links it to the one before, and each batch is closed by a block whose MAC
// is keyed from the chain's key (R-AUD-03). A [Verifier] recomputes the links and
// the MACs and reports every place where the stored rows disagree with them.
//
// The package imports no adapter and no database driver: the storage and the key
// access are the ports [Store] and [KeySource], implemented by the adapters and by
// the composition root.
package audit

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Format is the name and version of the public event document (orcon/audit@1).
const Format = "orcon/audit@1"

// Outcome is the result of an audited action. Its values are the public
// enumeration of orcon/audit@1 and keep their UPPER_SNAKE spelling (SPEC-004 section 21.8).
type Outcome string

// Outcomes of orcon/audit@1.
const (
	OutcomeSuccess Outcome = "SUCCESS"
	OutcomeFailure Outcome = "FAILURE"
	OutcomeDenied  Outcome = "DENIED"
)

// ActorKind says what kind of principal acted. Values are the public enumeration of orcon/audit@1.
type ActorKind string

// Actor kinds of orcon/audit@1.
const (
	ActorUser           ActorKind = "USER"
	ActorServiceAccount ActorKind = "SERVICE_ACCOUNT"
	ActorSCIMClient     ActorKind = "SCIM_CLIENT"
	ActorAPIKey         ActorKind = "API_KEY"
	ActorSystem         ActorKind = "SYSTEM"
)

// Valid reports whether k is one of the kinds of orcon/audit@1.
func (k ActorKind) Valid() bool {
	switch k {
	case ActorUser, ActorServiceAccount, ActorSCIMClient, ActorAPIKey, ActorSystem:
		return true
	default:
		return false
	}
}

// Actor is the principal that performed an action. The zero Actor is anonymous: a
// sign-in attempt that names no known user has no principal to record.
type Actor struct {
	PrincipalID id.UUID
	Kind        ActorKind
}

// IsAnonymous reports whether the actor is the zero Actor.
func (a Actor) IsAnonymous() bool { return a.PrincipalID.IsZero() && a.Kind == "" }

// Chain names the chain an entry belongs to: one per organization, and one for the
// instance (ADR-0077 section 5). The zero Chain is invalid.
type Chain struct {
	org      id.UUID
	instance bool
}

// OrganizationChain returns the chain of an organization.
func OrganizationChain(org id.UUID) Chain { return Chain{org: org} }

// InstanceChain returns the chain of the instance.
func InstanceChain() Chain { return Chain{instance: true} }

// IsInstance reports whether c is the instance chain.
func (c Chain) IsInstance() bool { return c.instance }

// Org returns the organization of an organization chain; the zero UUID for the instance chain.
func (c Chain) Org() id.UUID { return c.org }

// Valid reports whether c names exactly one chain.
func (c Chain) Valid() bool {
	if c.instance {
		return c.org.IsZero()
	}
	return !c.org.IsZero()
}

// Label names the chain inside the MAC input, so a block of one chain never
// verifies as a block of another.
func (c Chain) Label() string {
	if c.instance {
		return "instance"
	}
	return "org:" + c.org.String()
}

// Kind returns the chain's name in the public format: ORGANIZATION or INSTANCE.
func (c Chain) Kind() string {
	if c.instance {
		return "INSTANCE"
	}
	return "ORGANIZATION"
}

// Entry is one audited action, before sealing. Body is the canonical JSON
// (RFC 8785) of the event document without its sequence and hash; the hash of the
// sealed event covers these exact bytes, so the document is never re-serialized.
type Entry struct {
	ID         id.UUID
	Chain      Chain
	OccurredAt time.Time
	Actor      Actor
	Action     string
	Outcome    Outcome
	Body       []byte
}

// EntryParams is what an action supplies to [NewEntry].
type EntryParams struct {
	ID         id.UUID
	Chain      Chain
	OccurredAt time.Time
	// Actor is the zero Actor for an anonymous attempt.
	Actor   Actor
	Action  string
	Outcome Outcome
	// RequestID correlates the entry with the API request that caused it; optional.
	RequestID string
	// Reason is a short, non-secret explanation (for example a denial reason); optional.
	Reason string
	// Details holds the action's own fields. It must hold no secret and no execution data
	// (R-AUD-08); the zero value means none.
	Details jsonvalue.Value
}

// Errors of the audit package.
var (
	// ErrInvalidEntry means an entry violates the format rules; the message names the rule.
	ErrInvalidEntry = errors.New("audit: invalid entry")
	// ErrTailMoved means the chain was sealed by someone else since the sealer read its tail.
	ErrTailMoved = errors.New("audit: the chain moved since it was read")
)

// NewEntry validates the parameters and builds the entry with its canonical body.
// The occurrence time is truncated to microseconds, the precision the store keeps.
func NewEntry(p EntryParams) (Entry, error) {
	if p.ID.IsZero() {
		return Entry{}, fmt.Errorf("%w: the id is required", ErrInvalidEntry)
	}
	if !p.Chain.Valid() {
		return Entry{}, fmt.Errorf("%w: the chain is required", ErrInvalidEntry)
	}
	if p.OccurredAt.IsZero() {
		return Entry{}, fmt.Errorf("%w: the occurrence time is required", ErrInvalidEntry)
	}
	if !ValidAction(p.Action) {
		return Entry{}, fmt.Errorf("%w: the action name is not an identifier", ErrInvalidEntry)
	}
	switch p.Outcome {
	case OutcomeSuccess, OutcomeFailure, OutcomeDenied:
	default:
		return Entry{}, fmt.Errorf("%w: the outcome must be SUCCESS, FAILURE or DENIED", ErrInvalidEntry)
	}
	if !p.Actor.IsAnonymous() && (p.Actor.PrincipalID.IsZero() || !p.Actor.Kind.Valid()) {
		return Entry{}, fmt.Errorf("%w: an actor needs a principal and a kind", ErrInvalidEntry)
	}
	if len(p.RequestID) > 100 || len(p.Reason) > 500 {
		return Entry{}, fmt.Errorf("%w: request id or reason too long", ErrInvalidEntry)
	}
	at := p.OccurredAt.UTC().Truncate(time.Microsecond)
	body, err := buildBody(p, at)
	if err != nil {
		return Entry{}, err
	}
	if len(body) > MaxBodyBytes {
		return Entry{}, fmt.Errorf("%w: the document exceeds %d bytes", ErrInvalidEntry, MaxBodyBytes)
	}
	return Entry{
		ID: p.ID, Chain: p.Chain, OccurredAt: at, Actor: p.Actor, Action: p.Action,
		Outcome: p.Outcome, Body: body,
	}, nil
}

// MaxBodyBytes bounds the canonical document of one entry (the CHECK of the audit tables).
const MaxBodyBytes = 65536

func buildBody(p EntryParams, at time.Time) ([]byte, error) {
	members := []jsonvalue.Member{
		{Key: "format", Value: jsonvalue.String(Format)},
		{Key: "id", Value: jsonvalue.String(p.ID.String())},
		{Key: "chain", Value: jsonvalue.String(p.Chain.Kind())},
		{Key: "occurredAt", Value: jsonvalue.String(at.Format(timeLayout))},
		{Key: "action", Value: jsonvalue.String(p.Action)},
		{Key: "outcome", Value: jsonvalue.String(string(p.Outcome))},
	}
	if !p.Chain.IsInstance() {
		members = append(members, jsonvalue.Member{Key: "orgId", Value: jsonvalue.String(p.Chain.Org().String())})
	}
	if !p.Actor.IsAnonymous() {
		actor, err := jsonvalue.Object(
			jsonvalue.Member{Key: "kind", Value: jsonvalue.String(string(p.Actor.Kind))},
			jsonvalue.Member{Key: "principalId", Value: jsonvalue.String(p.Actor.PrincipalID.String())},
		)
		if err != nil {
			return nil, fmt.Errorf("%w: actor: %w", ErrInvalidEntry, err)
		}
		members = append(members, jsonvalue.Member{Key: "actor", Value: actor})
	}
	if p.RequestID != "" {
		members = append(members, jsonvalue.Member{Key: "requestId", Value: jsonvalue.String(p.RequestID)})
	}
	if p.Reason != "" {
		members = append(members, jsonvalue.Member{Key: "reason", Value: jsonvalue.String(p.Reason)})
	}
	details := p.Details
	if details.IsZero() {
		empty, err := jsonvalue.Object()
		if err != nil {
			return nil, fmt.Errorf("%w: details: %w", ErrInvalidEntry, err)
		}
		details = empty
	}
	members = append(members, jsonvalue.Member{Key: "details", Value: details})
	doc, err := jsonvalue.Object(members...)
	if err != nil {
		return nil, fmt.Errorf("%w: document: %w", ErrInvalidEntry, err)
	}
	body, err := jsonvalue.Canonical(doc)
	if err != nil {
		return nil, fmt.Errorf("%w: canonical form: %w", ErrInvalidEntry, err)
	}
	return body, nil
}

// timeLayout is RFC 3339 with microseconds, always in UTC.
const timeLayout = "2006-01-02T15:04:05.000000Z07:00"

// ValidAction reports whether an action name is an identifier: "category.action" or a
// single word such as "logout" (Annex B, auth). Segments hold lowercase letters, digits and
// underscores and are separated by single dots. The names keep their form in the public
// format (SPEC-004 section 21.8).
func ValidAction(name string) bool {
	if len(name) < 3 || len(name) > 100 {
		return false
	}
	prevDot := true
	for i := range len(name) {
		c := name[i]
		switch {
		case c == '.':
			if prevDot || i == len(name)-1 {
				return false
			}
			prevDot = true
		case (c >= 'a' && c <= 'z') || c == '_' || (c >= '0' && c <= '9'):
			prevDot = false
		default:
			return false
		}
	}
	return true
}

// sequenceBytes encodes a sequence number as eight big-endian bytes. Sequence numbers
// start at 1, so a negative value (which no chain holds) encodes as zero.
func sequenceBytes(n int64) [8]byte {
	var b [8]byte
	if n < 0 {
		return b
	}
	binary.BigEndian.PutUint64(b[:], uint64(n))
	return b
}

// chainHash is SHA-256 over the previous hash, the sequence number as eight big-endian
// bytes, and the canonical body. Binding the sequence number makes a reordered
// event fail even when its neighbors are intact.
func chainHash(prev [32]byte, sequence int64, body []byte) [32]byte {
	seq := sequenceBytes(sequence)
	h := sha256.New()
	h.Write(prev[:])
	h.Write(seq[:])
	h.Write(body)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
