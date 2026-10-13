// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package store is inside the data-access layer: driver calls are allowed, but
// the statement must be a constant.
package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"fixture/internal/adapter/sqlite/helper"
)

const getFlow = "SELECT id, name FROM flow WHERE org_id = ? AND id = ?"

// A constant assembled from constants is still a constant.
const listFlows = "SELECT id, name " +
	"FROM flow WHERE org_id = ?"

type queries struct{ query string }

func constants(ctx context.Context, db *sql.DB) {
	_, _ = db.ExecContext(ctx, getFlow, 1, 2)
	_, _ = db.QueryContext(ctx, listFlows, 1)
	_, _ = db.QueryContext(ctx, "SELECT 1"+" FROM flow")
	const local = "SELECT 2"
	_, _ = db.QueryContext(ctx, local)
}

func notConstants(ctx context.Context, db *sql.DB, table, query string, q queries, parts []string, prepare func() string) {
	_, _ = db.QueryContext(ctx, "SELECT id FROM "+table)                          // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM flow", table))        // want `SQL text must be a constant`
	_, _ = db.ExecContext(ctx, strings.Join([]string{"DELETE FROM", table}, " ")) // want `SQL text must be a constant`
	_, _ = db.ExecContext(ctx, ("DELETE FROM " + table))                          // want `SQL text must be a constant`
	_, _ = db.PrepareContext(ctx, strings.ReplaceAll(getFlow, "flow", table))     // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, query)                                            // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, q.query)                                          // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, parts[0])                                         // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, prepare())                                        // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, helper.Select(table))                             // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, helper.Trim())                                    // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, strings.TrimSpace(getFlow))                       // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, errors.New("SELECT 1").Error())                   // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, fmt.Errorf("SELECT %s", table).Error())           // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, func() string { return "SELECT 1" }())            // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, getText(table))                                   // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, generic("SELECT 1"))                              // want `SQL text must be a constant`

	built := "SELECT id FROM " + table
	_, _ = db.QueryContext(ctx, built) // want `SQL text must be a constant`

	extended := "SELECT id"
	extended += " FROM flow"
	_, _ = db.QueryContext(ctx, extended) // want `SQL text must be a constant`

	var buf bytes.Buffer
	_, _ = db.QueryContext(ctx, string(buf.Bytes())) // want `SQL text must be a constant`
	_, _ = db.QueryContext(ctx, buf.String())        // want `SQL text must be a constant`

	var sb strings.Builder
	_, _ = db.QueryContext(ctx, sb.String()) // want `SQL text must be a constant`

	method := db.QueryContext
	_, _ = method(ctx, built)
}

func getText(table string) string { return "SELECT id FROM " + table }

func generic[T ~string](s T) T { return s }

// The directive is the reviewed way to run text the code did not write.
func reviewed(ctx context.Context, db *sql.DB, embedded string) {
	//orcon:dynamic-sql runs one migration file embedded in the binary
	_, _ = db.ExecContext(ctx, embedded)
	_, _ = db.ExecContext(ctx, embedded) //orcon:dynamic-sql same, on the call line
	//orcon:dynamic-sql
	_, _ = db.ExecContext(ctx, embedded) // want `needs a reason`
	_, _ = db.ExecContext(ctx, embedded) // want `SQL text must be a constant`
}
