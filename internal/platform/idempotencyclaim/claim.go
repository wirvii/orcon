// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package idempotencyclaim carries the idempotency claim of a request from the HTTP
// pipeline to the store that performs the change (ADR-0053 section 4, SPEC-005 Amendment 8
// section 8-2). The pipeline puts the claim in the context; the store writes the record in
// the transaction of the change, so a committed change always has its record. The package
// imports nothing of the product: it is a leaf of the platform.
package idempotencyclaim

import (
	"context"
	"errors"

	"github.com/wirvii/orcon/internal/platform/id"
)

// ErrTaken means another request already holds the key: the record exists and is live. The
// caller answers 409 idempotency-in-progress, or replays the stored response once it completes.
var ErrTaken = errors.New("the idempotency key is already in use")

// Claim is the key a request claimed and the fingerprint of its content. The store reads it
// from the context and writes the record in the transaction of the change.
type Claim struct {
	// ID identifies the record the transaction writes; the caller creates it (orcon-db ID-1).
	ID id.UUID
	// Requester is the exact requesting identity of the key.
	Requester string
	// Value is the Idempotency-Key header.
	Value string
	// Fingerprint is the SHA-256 of the request.
	Fingerprint [32]byte
}

type claimKey struct{}

// With returns a context that carries c.
func With(ctx context.Context, c Claim) context.Context {
	return context.WithValue(ctx, claimKey{}, c)
}

// From returns the claim of the context, when one was made.
func From(ctx context.Context) (Claim, bool) {
	c, ok := ctx.Value(claimKey{}).(Claim)
	return c, ok
}
