// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/id"
)

// micros is the stored form of an instant: Unix microseconds in UTC
// (orcon-db TY-4).
func micros(t time.Time) int64 { return t.UTC().UnixMicro() }

// instant converts stored microseconds back to a UTC time.
func instant(v int64) time.Time { return time.UnixMicro(v).UTC() }

// nullInstant converts a nullable stored instant; NULL becomes the zero time.
func nullInstant(v sql.NullInt64) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return instant(v.Int64)
}

// uuidLike is every identity type of the areas: an array of 16 bytes.
type uuidLike interface{ ~[16]byte }

// uuidBytes converts a required identity to its 16 stored bytes.
func uuidBytes[T uuidLike](u T) []byte {
	b := [16]byte(u)
	return b[:]
}

// nullUUIDBytes converts an optional identity: the zero value becomes NULL.
func nullUUIDBytes[T uuidLike](u T) []byte {
	if u == (T{}) {
		return nil
	}
	return uuidBytes(u)
}

// uuidOf converts stored bytes to an identity type.
func uuidOf[T uuidLike](b []byte) (T, error) {
	u, err := id.FromBytes(b)
	return T(u), err
}

// nullUUIDOf converts a nullable identity; NULL becomes the zero value.
func nullUUIDOf[T uuidLike](b []byte) (T, error) {
	if b == nil {
		return T{}, nil
	}
	return uuidOf[T](b)
}

// nullText converts an optional text: the empty string becomes NULL.
func nullText(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// nullMicros converts an optional instant: the zero time becomes NULL.
func nullMicros(t time.Time) sql.NullInt64 {
	if t.IsZero() {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: micros(t), Valid: true}
}

// mapError turns a driver error into an area error. Constraint violations map
// by class to the errors of the flow area (orcon-db IN-8); the original error
// stays in the chain for diagnosis.
func mapError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, flow.ErrNotFound)
	}
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return fmt.Errorf("%s: %w: %w", op, flow.ErrConflict, err)
		case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
			return fmt.Errorf("%s: %w: %w", op, flow.ErrReference, err)
		case sqlite3.SQLITE_CONSTRAINT_CHECK, sqlite3.SQLITE_CONSTRAINT_NOTNULL:
			return fmt.Errorf("%s: %w: %w", op, flow.ErrInvalidRecord, err)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }
