// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package reviewed is listed in the allowlist, so the directive is honoured.
package reviewed

import (
	"context"
	"database/sql"
)

// Run executes one embedded file.
func Run(ctx context.Context, db *sql.DB, embedded string) {
	//orcon:dynamic-sql runs one migration file embedded in the binary
	_, _ = db.ExecContext(ctx, embedded)
	_, _ = db.ExecContext(ctx, embedded) //orcon:dynamic-sql same, on the call line
	//orcon:dynamic-sql
	_, _ = db.ExecContext(ctx, embedded) // want `needs a reason`
	_, _ = db.ExecContext(ctx, embedded) // want `SQL text must be a constant`
}
