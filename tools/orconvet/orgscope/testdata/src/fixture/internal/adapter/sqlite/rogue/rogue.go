// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package rogue carries a copy of the allowlist next to itself listing itself;
// only the allowlist at the repository root counts.
package rogue

import (
	"context"
	"database/sql"
)

// Run tries to use the directive.
func Run(ctx context.Context, db *sql.DB, text string) {
	//orcon:dynamic-sql self-authorized // want `honoured only in the files of`
	_, _ = db.ExecContext(ctx, text) // want `SQL text must be a constant`
}
