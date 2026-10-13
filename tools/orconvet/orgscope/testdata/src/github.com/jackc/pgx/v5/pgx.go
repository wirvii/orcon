// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package pgx is a stub of the driver for the analyzer fixtures.
package pgx

import "context"

// Conn is a stub connection.
type Conn struct{}

// Exec is a stub.
func (*Conn) Exec(_ context.Context, _ string, _ ...any) error { return nil }

// Query is a stub.
func (*Conn) Query(_ context.Context, _ string, _ ...any) error { return nil }
