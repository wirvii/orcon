// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package sqlcgen stands for the generated code of the PostgreSQL adapter.
package sqlcgen

import "context"

// DBTX is the interface sqlc generates for the database handle.
type DBTX interface {
	Exec(ctx context.Context, query string, args ...any) error
}
