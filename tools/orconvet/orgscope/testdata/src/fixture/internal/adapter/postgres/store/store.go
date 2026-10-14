// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package store calls the generated database handle and pgx in the PostgreSQL adapter.
package store

import (
	"context"

	"github.com/jackc/pgx/v5"

	"fixture/internal/adapter/postgres/sqlcgen"
)

const deleteFlow = "DELETE FROM flow WHERE org_id = $1 AND id = $2"

func calls(ctx context.Context, tx sqlcgen.DBTX, conn *pgx.Conn, batch *pgx.Batch, table, name string) {
	_ = tx.Exec(ctx, deleteFlow, 1, 2)
	_ = tx.Exec(ctx, "DELETE FROM "+table) // want `SQL text must be a constant`

	_ = conn.Prepare(ctx, name, deleteFlow)      // the name of a prepared statement may vary
	_ = conn.Prepare(ctx, "by_org", deleteFlow)  // constant text
	_ = conn.Prepare(ctx, "by_org", table)       // want `SQL text must be a constant`
	_ = conn.Prepare(ctx, name, "SELECT "+table) // want `SQL text must be a constant`
	batch.Queue("DELETE FROM " + table)          // want `SQL text must be a constant`
	batch.Queue(deleteFlow, 1, 2)
}
