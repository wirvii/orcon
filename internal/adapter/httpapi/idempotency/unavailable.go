// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package idempotency

import (
	"context"
	"crypto/sha256"
	"errors"
)

// ErrStoreUnavailable means there is no store that can keep idempotency records.
// The pipeline answers 503 service-unavailable, so no request that asked for
// idempotency runs without it.
var ErrStoreUnavailable = errors.New("the idempotency store is unavailable")

// UnavailableStore is the store of a process that has no durable one yet. It
// refuses every claim, so an operation that takes an Idempotency-Key answers 503
// instead of running without the protection the key promises.
type UnavailableStore struct{}

// Begin implements [Store].
func (UnavailableStore) Begin(context.Context, Key, [sha256.Size]byte) (Record, bool, error) {
	return Record{}, false, ErrStoreUnavailable
}

// Complete implements [Store].
func (UnavailableStore) Complete(context.Context, Key, *Response) error { return ErrStoreUnavailable }

// Abandon implements [Store].
func (UnavailableStore) Abandon(context.Context, Key) error { return ErrStoreUnavailable }
