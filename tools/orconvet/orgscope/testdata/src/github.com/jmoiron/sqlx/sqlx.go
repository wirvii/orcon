// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package sqlx is a stub of the wrapper for the analyzer fixtures.
package sqlx

// DB is a stub.
type DB struct{}

// Get is a stub.
func (*DB) Get(dest any, query string, _ ...any) error { return nil }
