// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"errors"
	"fmt"
)

// Errors the stores return. Adapters map constraint violations to them by
// constraint name (orcon-db IN-8). The flow area re-exports the first five, so
// one failure satisfies errors.Is for both areas.
var (
	// ErrNotFound means no row matches, or the row belongs to another organization.
	ErrNotFound = errors.New("not found")
	// ErrConflict means a uniqueness rule rejected the write: a duplicate name,
	// slug, version number, content hash or identity.
	ErrConflict = errors.New("conflict")
	// ErrReference means a referenced row does not exist, or a row that is still
	// referenced cannot be removed.
	ErrReference = errors.New("referenced row missing or still in use")
	// ErrClosed means the store was closed; an operation that was in flight when
	// Close was called finishes first, one that starts after it fails with this.
	ErrClosed = errors.New("store is closed")
	// ErrInvalidRecord means a database constraint rejected the values
	// (a CHECK: unknown enumeration value, empty name, wrong format tag).
	ErrInvalidRecord = errors.New("record violates a database constraint")
)

// ErrNoScope means a store method was called with the zero organization scope,
// which scopes to nothing.
var ErrNoScope = errors.New("the organization scope is empty")

// ErrScopeMismatch means a record names an organization other than the scope it
// is written under.
var ErrScopeMismatch = errors.New("the record belongs to another organization than the scope")

// ErrHandleTaken is what a store returns when the project handle is already held
// by another project of the instance; it also satisfies errors.Is for
// [ErrConflict]. [Projects.Create] never shows it to a caller: see
// [ErrHandleUnavailable].
var ErrHandleTaken = fmt.Errorf("project handle is taken: %w", ErrConflict)

// ErrHandleUnavailable is what a caller of [Projects.Create] gets when no handle
// could be assigned. Its text never names a flow, a project or an organization of
// anyone, whoever holds the handle (SPEC-004 R-ID-11, owner decision Q11).
var ErrHandleUnavailable = errors.New("project identifier is not available")

// ErrInvalidInput means a value failed validation before reaching the store.
var ErrInvalidInput = errors.New("invalid input")
