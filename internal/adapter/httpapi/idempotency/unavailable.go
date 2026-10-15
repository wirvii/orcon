// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package idempotency

import (
	"context"
	"errors"
	"time"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// ErrStoreUnavailable means there is no store that can keep idempotency records. The
// pipeline answers 503 service-unavailable, so no request that asked for idempotency runs
// without the protection the key promises.
var ErrStoreUnavailable = errors.New("the idempotency store is unavailable")

// UnavailableStore is the store of a process that has no durable one. It refuses every
// read and write, so an operation that takes an Idempotency-Key answers 503 instead of
// running without the protection the key promises.
type UnavailableStore struct{}

var _ identity.IdempotencyStore = UnavailableStore{}

// LookupIdempotency implements [identity.IdempotencyStore].
func (UnavailableStore) LookupIdempotency(context.Context, orgscope.Org, identity.IdempotencyKey, time.Time) (identity.IdempotencyRecord, bool, error) {
	return identity.IdempotencyRecord{}, false, ErrStoreUnavailable
}

// CompleteIdempotency implements [identity.IdempotencyStore].
func (UnavailableStore) CompleteIdempotency(context.Context, orgscope.Org, identity.IdempotencyKey, identity.IdempotencyCompletion, time.Time) error {
	return ErrStoreUnavailable
}
