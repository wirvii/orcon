// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package store calls the generated database handle of the PostgreSQL adapter.
package store

import (
	"context"

	"fixture/internal/adapter/postgres/sqlcgen"
)

const deleteFlow = "DELETE FROM flow WHERE org_id = $1 AND id = $2"

func calls(ctx context.Context, tx sqlcgen.DBTX, table string) {
	_ = tx.Exec(ctx, deleteFlow, 1, 2)
	_ = tx.Exec(ctx, "DELETE FROM "+table) // want `SQL text is built at run time`
}
