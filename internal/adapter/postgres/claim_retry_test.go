// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestAbortedByDatabase pins which errors a claim runs again: only the two aborts of SQLSTATE class
// 40, also when the adapter has wrapped them. The other errors must reach the caller at once.
func TestAbortedByDatabase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "deadlock", err: &pgconn.PgError{Code: codeDeadlockDetected}, want: true},
		{name: "serialization failure", err: &pgconn.PgError{Code: codeSerializationFailure}, want: true},
		{name: "deadlock wrapped by the adapter", err: mapError("claim tokens", &pgconn.PgError{Code: codeDeadlockDetected}), want: true},
		{name: "unique violation is not retried", err: &pgconn.PgError{Code: codeUnique}, want: false},
		{name: "connection error is not retried", err: errors.New("connection reset"), want: false},
		{name: "no error", err: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := abortedByDatabase(tt.err); got != tt.want {
				t.Fatalf("abortedByDatabase(%v) = %t, want %t", tt.err, got, tt.want)
			}
		})
	}
}
