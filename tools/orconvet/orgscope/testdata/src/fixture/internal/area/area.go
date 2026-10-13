// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package area is a business area; SQL here is reported.
package area

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
)

func standardLibrary(db *sql.DB, name string) { // want `SQL access \(sql.DB\) outside the data-access layer`
	_, _ = db.Exec("DELETE FROM flow")                          // want `SQL access \(sql.Exec\) outside the data-access layer`
	_, _ = db.Query("SELECT id FROM flow WHERE name = " + name) // want `SQL access \(sql.Query\) outside the data-access layer` `SQL text must be a constant`
	_, _ = sql.Open("sqlite", "file:x")                         // want `SQL access \(sql.Open\) outside the data-access layer`
	_ = sql.ErrNoRows                                           // want `SQL access \(sql.ErrNoRows\) outside the data-access layer`
}

func pgxDriver(ctx context.Context, conn *pgx.Conn, name string) { // want `SQL access \(pgx.Conn\) outside the data-access layer`
	_ = conn.Exec(ctx, "DELETE FROM flow")                        // want `SQL access \(pgx.Exec\) outside the data-access layer`
	_ = conn.Query(ctx, fmt.Sprintf("SELECT %s FROM flow", name)) // want `SQL access \(pgx.Query\) outside the data-access layer` `SQL text must be a constant`
}

func otherDrivers(db *sqlx.DB, drv sqlite3.SQLiteDriver) { // want `SQL access \(sqlx.DB\) outside the data-access layer` `SQL access \(sqlite3.SQLiteDriver\) outside the data-access layer`
	_ = pq.CopyIn("flow") // want `SQL access \(pq.CopyIn\) outside the data-access layer`
	_ = drv
}

// A method value and a function value carry the driver out of a call expression.
func indirect(db *sql.DB) { // want `SQL access \(sql.DB\) outside the data-access layer`
	run := db.Exec                 // want `SQL access \(sql.Exec\) outside the data-access layer`
	_, _ = run("DELETE FROM flow") // want `call returns a SQL driver value outside the data-access layer`
	pass(db.Query)                 // want `SQL access \(sql.Query\) outside the data-access layer`
}

func pass(func(string, ...any) (*sql.Rows, error)) {} // want `SQL access \(sql.Rows\) outside the data-access layer`

// A helper that hands out a driver value is caught at the call.
func hands(get func() *sql.DB) { // want `SQL access \(sql.DB\) outside the data-access layer`
	db := get() // want `call returns a SQL driver value outside the data-access layer`
	_ = db
}

// notSQL calls look-alike methods that are not drivers.
type notSQL struct{}

func (notSQL) Exec(query string) {}

func lookAlikes(name string) {
	notSQL{}.Exec("a" + name)
	_ = fmt.Sprintf("SELECT %s", name)
}
