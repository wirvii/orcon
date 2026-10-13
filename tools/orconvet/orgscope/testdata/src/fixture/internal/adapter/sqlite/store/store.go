// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package store is inside the data-access layer: driver calls are allowed,
// run-time query text is not.
package store

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const getFlow = "SELECT id, name FROM flow WHERE org_id = ? AND id = ?"

// A constant assembled from constants is still a constant.
const listFlows = "SELECT id, name " +
	"FROM flow WHERE org_id = ?"

func clean(ctx context.Context, db *sql.DB, query string) {
	_, _ = db.ExecContext(ctx, getFlow, 1, 2)
	_, _ = db.QueryContext(ctx, listFlows, 1)
	_, _ = db.QueryContext(ctx, "SELECT 1"+" FROM flow")
	_, _ = db.QueryContext(ctx, query) // a parameter says nothing about how it was built
	stmt := getFlow
	_, _ = db.QueryContext(ctx, stmt)
}

func dynamic(ctx context.Context, db *sql.DB, table, column string) {
	_, _ = db.QueryContext(ctx, "SELECT id FROM "+table)                          // want `SQL text is built at run time`
	_, _ = db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM flow", column))       // want `SQL text is built at run time`
	_, _ = db.ExecContext(ctx, strings.Join([]string{"DELETE FROM", table}, " ")) // want `SQL text is built at run time`
	_, _ = db.ExecContext(ctx, ("DELETE FROM " + table))                          // want `SQL text is built at run time`
	_, _ = db.PrepareContext(ctx, "SELECT id FROM "+table+" WHERE org_id = ?")    // want `SQL text is built at run time`
	_, _ = db.PrepareContext(ctx, strings.ReplaceAll(getFlow, "flow", table))     // want `SQL text is built at run time`

	built := "SELECT id FROM " + table
	_, _ = db.QueryContext(ctx, built) // want `SQL text is built at run time`

	var extended = "SELECT id"
	extended += " FROM flow"
	_, _ = db.QueryContext(ctx, extended) // want `SQL text is built at run time`

	var buf bytes.Buffer
	buf.WriteString("SELECT id FROM ")
	buf.WriteString(table)
	_, _ = db.QueryContext(ctx, buf.String()) // want `SQL text is built at run time`

	var sb strings.Builder
	sb.WriteString("SELECT id FROM flow")
	_, _ = db.QueryContext(ctx, sb.String()) // want `SQL text is built at run time`

	again := built
	_, _ = db.QueryContext(ctx, again) // want `SQL text is built at run time`
}
