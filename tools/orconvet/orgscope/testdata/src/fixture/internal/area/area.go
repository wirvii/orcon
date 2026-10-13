// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package area is a business area; SQL here is reported.
package area

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func standardLibrary(db *sql.DB, name string) {
	_, _ = db.Exec("DELETE FROM flow")                          // want `SQL access \(sql.Exec\) outside the data-access layer`
	_, _ = db.Query("SELECT id FROM flow WHERE name = " + name) // want `SQL access \(sql.Query\) outside the data-access layer` `SQL text is built at run time`
	_, _ = sql.Open("sqlite", "file:x")                         // want `SQL access \(sql.Open\) outside the data-access layer`
}

func pgxDriver(ctx context.Context, conn *pgx.Conn, name string) {
	_ = conn.Exec(ctx, "DELETE FROM flow")                        // want `SQL access \(pgx.Exec\) outside the data-access layer`
	_ = conn.Query(ctx, fmt.Sprintf("SELECT %s FROM flow", name)) // want `SQL access \(pgx.Query\) outside the data-access layer` `SQL text is built at run time`
}

// notSQL calls look-alike methods that are not drivers.
type notSQL struct{}

func (notSQL) Exec(query string) {}

func lookAlikes(name string) {
	notSQL{}.Exec("a" + name)
	_ = sql.ErrNoRows
	_ = fmt.Sprintf("SELECT %s", name)
}
