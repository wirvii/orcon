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

// uuidBytes converts a required identity to its 16 stored bytes.
func uuidBytes(u id.UUID) []byte { return u.Bytes() }

// nullUUIDBytes converts an optional identity: the zero UUID becomes NULL.
func nullUUIDBytes(u id.UUID) []byte {
	if u.IsZero() {
		return nil
	}
	return u.Bytes()
}

// uuidFrom converts stored bytes to an identity; NULL becomes the zero UUID.
func uuidFrom(b []byte) (id.UUID, error) {
	if b == nil {
		return id.Nil, nil
	}
	return id.FromBytes(b)
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
