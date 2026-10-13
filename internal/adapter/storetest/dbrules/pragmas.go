// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// SQLiteRole says what a SQLite connection is for.
type SQLiteRole string

const (
	// SQLiteWriter is the connection of the single writer.
	SQLiteWriter SQLiteRole = "writer"
	// SQLiteReader is a connection of the reader pool.
	SQLiteReader SQLiteRole = "reader"
)

// SQLiteOption adjusts CheckSQLiteConnection.
type SQLiteOption func(*sqliteOptions)

type sqliteOptions struct {
	synchronous []int
}

// AllowSynchronous accepts the given PRAGMA synchronous value on the writer
// besides FULL (2): the explicitly configured, documented weaker setting
// (connections.md CN-3).
func AllowSynchronous(mode int) SQLiteOption {
	return func(o *sqliteOptions) { o.synchronous = append(o.synchronous, mode) }
}

// CheckSQLiteConnection verifies the per-connection settings of one SQLite
// connection (connections.md CN-2 to CN-6): foreign_keys = 1, busy_timeout of
// at least 1000 ms, trusted_schema = 0; on the writer journal_mode = wal and
// synchronous = FULL (or an allowed value); on a reader query_only = 1.
// Settings that cannot be read are findings.
func CheckSQLiteConnection(ctx context.Context, conn *sql.Conn, role SQLiteRole, opts ...SQLiteOption) []Finding {
	o := sqliteOptions{synchronous: []int{2}}
	for _, opt := range opts {
		opt(&o)
	}
	var out []Finding
	pragma := func(name string) (string, bool) {
		var v string
		if err := conn.QueryRowContext(ctx, "PRAGMA "+name).Scan(&v); err != nil {
			out = append(out, Finding{Rule: "CN-2", Object: "pragma " + name, Message: "cannot be read: " + err.Error()})
			return "", false
		}
		return v, true
	}
	if v, ok := pragma("foreign_keys"); ok && v != "1" {
		out = append(out, Finding{Rule: "CN-2", Object: "pragma foreign_keys", Message: "must be 1 on every connection (found " + v + "); set it through the driver's per-connection parameters"})
	}
	if v, ok := pragma("busy_timeout"); ok {
		if n, err := strconv.Atoi(v); err != nil || n < 1000 {
			out = append(out, Finding{Rule: "CN-2", Object: "pragma busy_timeout", Message: "must be at least 1000 milliseconds (found " + v + ")"})
		}
	}
	if v, ok := pragma("trusted_schema"); ok && v != "0" {
		out = append(out, Finding{Rule: "CN-2", Object: "pragma trusted_schema", Message: "must be 0 (found " + v + ")"})
	}
	switch role {
	case SQLiteWriter:
		if v, ok := pragma("journal_mode"); ok && !strings.EqualFold(v, "wal") {
			out = append(out, Finding{Rule: "CN-3", Object: "pragma journal_mode", Message: "the writer uses wal (found " + v + ")"})
		}
		if v, ok := pragma("synchronous"); ok {
			n, err := strconv.Atoi(v)
			if err != nil || !containsInt(o.synchronous, n) {
				out = append(out, Finding{Rule: "CN-3", Object: "pragma synchronous", Message: fmt.Sprintf("the writer uses FULL (2) unless a weaker value is configured explicitly (found %s)", v)})
			}
		}
	case SQLiteReader:
		if v, ok := pragma("query_only"); ok && v != "1" {
			out = append(out, Finding{Rule: "CN-4", Object: "pragma query_only", Message: "readers are query_only (found " + v + ")"})
		}
	default:
		out = append(out, Finding{Rule: "CN-2", Object: "role " + string(role), Message: "unknown connection role; use SQLiteWriter or SQLiteReader"})
	}
	return out
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

var zeroTimeoutRE = regexp.MustCompile(`^0+(\.0+)?\s*(us|ms|s|min|h|d)?$`)

// CheckPostgresSession verifies the settings of a PostgreSQL transaction at
// its start (connections.md CN-7, CN-8, tenant-isolation.md TI-6): TimeZone =
// UTC, current_user = orcon_application, an empty orcon.org_id, read committed
// isolation and non-zero idle_in_transaction_session_timeout and lock_timeout.
func CheckPostgresSession(ctx context.Context, tx pgx.Tx) []Finding {
	var out []Finding
	read := func(query string) (string, bool) {
		var v *string
		if err := tx.QueryRow(ctx, query).Scan(&v); err != nil {
			out = append(out, Finding{Rule: "CN-7", Object: query, Message: "cannot be read: " + err.Error()})
			return "", false
		}
		if v == nil {
			return "", true
		}
		return *v, true
	}
	if v, ok := read(`SELECT current_setting('TimeZone')`); ok && v != "UTC" {
		out = append(out, Finding{Rule: "CN-7", Object: "setting TimeZone", Message: "must be UTC (found " + v + ")"})
	}
	if v, ok := read(`SELECT current_user::text`); ok && v != applicationRole {
		out = append(out, Finding{Rule: "CN-8", Object: "current_user", Message: "must be " + applicationRole + " (found " + v + ")"})
	}
	if v, ok := read(`SELECT current_setting('orcon.org_id', true)`); ok && v != "" {
		out = append(out, Finding{Rule: "TI-6", Object: "setting orcon.org_id", Message: "the organization context is empty when a transaction starts (found " + v + ")"})
	}
	if v, ok := read(`SELECT current_setting('transaction_isolation')`); ok && v != "read committed" {
		out = append(out, Finding{Rule: "CN-7", Object: "transaction_isolation", Message: "must be read committed (found " + v + ")"})
	}
	for _, name := range []string{"idle_in_transaction_session_timeout", "lock_timeout"} {
		if v, ok := read(`SELECT current_setting('` + name + `')`); ok && (v == "" || zeroTimeoutRE.MatchString(v)) {
			out = append(out, Finding{Rule: "CN-7", Object: "setting " + name, Message: "must not be disabled (found " + orDefault(v, "empty") + ")"})
		}
	}
	return out
}
