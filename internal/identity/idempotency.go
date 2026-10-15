// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// IdempotencyKey identifies one idempotent request within an organization: the exact
// requesting identity (the session, key or token, so two keys of one user never share a
// record) and the Idempotency-Key the client sent (ADR-0053 section 4).
type IdempotencyKey struct {
	Requester string
	Value     string
}

// IdempotencyResponse is what a completed request answered, as far as a replay needs it.
type IdempotencyResponse struct {
	Status      int
	ContentType string
	Location    string
	ETag        string
	Body        []byte
	// SecretIssued is true when the response disclosed a secret. Its body is not stored, and a
	// repeat is refused with 409 secret-already-issued (SPEC-012 FIELD-7).
	SecretIssued bool
}

// IdempotencyRecord is the stored state of a key.
type IdempotencyRecord struct {
	// Fingerprint is the SHA-256 of the request that claimed the key.
	Fingerprint [32]byte
	// Completed is false between the commit of the change and the storing of its response.
	Completed bool
	// ResourceRef names the resource the change created ("project:<id>"). It is written in the
	// transaction of the change, so an in-progress record proves that the change committed.
	ResourceRef string
	// Response is valid when Completed.
	Response IdempotencyResponse
	// CreatedAt is when the key was claimed; it decides whether an in-progress record is stale.
	CreatedAt time.Time
}

// IdempotencyTTL is how long a record is kept (ADR-0053 section 4).
const IdempotencyTTL = 24 * time.Hour

// IdempotencyCompletion is what completing a key stores: the identity of the record, which the
// caller creates (orcon-db ID-1), the fingerprint of the request and its response.
type IdempotencyCompletion struct {
	ID          id.UUID
	Fingerprint [32]byte
	Response    IdempotencyResponse
}

// IdempotencyStore keeps the idempotency records. The claim of a key is not a method of
// this port: an operation that changes state claims its key in the same transaction as the
// change, the claim being carried by the context (internal/platform/idempotencyclaim), so a
// change and its record commit together or not at all.
type IdempotencyStore interface {
	// LookupIdempotency returns the record of key in the organization and whether one
	// exists. A record past its expiry is reported as absent.
	LookupIdempotency(ctx context.Context, org orgscope.Org, key IdempotencyKey, now time.Time) (IdempotencyRecord, bool, error)
	// CompleteIdempotency stores the response of key. It completes the record that a transaction
	// claimed; when no record of key is in progress, it stores a completed one with the fingerprint.
	// The second case is a success whose operation did not claim its key in its transaction: the
	// replay works, but a crash between the change and this call can repeat the change, so an
	// operation that changes state claims its key (see the package comment).
	CompleteIdempotency(ctx context.Context, org orgscope.Org, key IdempotencyKey, c IdempotencyCompletion, now time.Time) error
}
