// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package pgx is a stub of the driver for the analyzer fixtures.
package pgx

import "context"

// Conn is a stub connection.
type Conn struct{}

// Exec is a stub.
func (*Conn) Exec(_ context.Context, sql string, _ ...any) error { return nil }

// Query is a stub.
func (*Conn) Query(_ context.Context, sql string, _ ...any) error { return nil }

// Prepare is a stub; the first string is the name of the statement, the second its text.
func (*Conn) Prepare(_ context.Context, name string, sql string) error { return nil }

// Batch is a stub.
type Batch struct{}

// Queue is a stub.
func (*Batch) Queue(query string, _ ...any) {}
