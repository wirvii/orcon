// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// timestamp converts an instant to the stored form: UTC, truncated to
// microseconds (orcon-db TY-4).
func timestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC().Truncate(time.Microsecond), Valid: true}
}

// instant converts a stored instant back to UTC; NULL becomes the zero time.
func instant(ts pgtype.Timestamptz) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time.UTC()
}

// uuidLike is every identity type of the areas: an array of 16 bytes.
type uuidLike interface{ ~[16]byte }

// pgUUID converts a required identity.
func pgUUID[T uuidLike](u T) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte(u), Valid: true} }

// nullPGUUID converts an optional identity: the zero value becomes NULL.
func nullPGUUID[T uuidLike](u T) pgtype.UUID {
	if u == (T{}) {
		return pgtype.UUID{}
	}
	return pgUUID(u)
}

// fromPGUUID converts a stored identity; NULL becomes the zero value.
func fromPGUUID[T uuidLike](u pgtype.UUID) T {
	if !u.Valid {
		return T{}
	}
	return T(u.Bytes)
}

// nullText converts an optional text: the empty string becomes NULL.
func nullText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// nullTimestamp converts an optional instant: the zero time becomes NULL.
func nullTimestamp(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return timestamp(t)
}

// handleIndex is the unique index of the project handle.
const handleIndex = "uq_project_handle"

// Violation classes of SQLSTATE (PostgreSQL appendix A).
const (
	codeNotNull    = "23502"
	codeForeignKey = "23503"
	codeUnique     = "23505"
	codeCheck      = "23514"
)

// mapError turns a driver error into an area error. Constraint violations map
// by SQLSTATE class to the errors of the flow area (orcon-db IN-8); the
// original error stays in the chain for diagnosis.
func mapError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, flow.ErrNotFound)
	}
	if errors.Is(err, flow.ErrDraftConflict) || errors.Is(err, flow.ErrNotFound) || errors.Is(err, flow.ErrVersionNotFound) ||
		errors.Is(err, flow.ErrSnapshotNotFound) || errors.Is(err, ErrClosed) ||
		errors.Is(err, identity.ErrNoScope) || errors.Is(err, identity.ErrScopeMismatch) {
		return fmt.Errorf("%s: %w", op, err)
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case codeUnique:
			if pe.ConstraintName == handleIndex {
				return fmt.Errorf("%s: %w: %w", op, identity.ErrHandleTaken, err)
			}
			return fmt.Errorf("%s: %w: %w", op, flow.ErrConflict, err)
		case codeForeignKey:
			return fmt.Errorf("%s: %w: %w", op, flow.ErrReference, err)
		case codeCheck, codeNotNull:
			return fmt.Errorf("%s: %w: %w", op, flow.ErrInvalidRecord, err)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

// maxPage is the largest page a store returns (orcon-db QY-6).
const maxPage = 1000

func clampPage(limit int) int32 {
	switch {
	case limit < 1:
		return 1
	case limit > maxPage:
		return maxPage
	default:
		return int32(limit)
	}
}

func notFoundAs(err, target error) error {
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: %w", target, flow.ErrNotFound)
	}
	return err
}

func errorsIsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func conflictf(format string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), flow.ErrConflict)
}

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), flow.ErrInvalidRecord)
}
