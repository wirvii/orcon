// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
)

var sqliteScanRE = regexp.MustCompile(`^SCAN (?:TABLE )?(\S+)(?: AS \S+)?(.*)$`)

// CheckPlansSQLite runs EXPLAIN QUERY PLAN on every hot query (queries.go,
// -- orcon:hot) with unbound parameters and reports a full table scan and, for
// list queries with ORDER BY, a temporary sort (queries rule QY-4, checks.md
// CK-9). A scan that uses an index (USING INDEX, USING COVERING INDEX or USING
// INTEGER PRIMARY KEY) passes. The schema must be applied first.
func CheckPlansSQLite(ctx context.Context, db *sql.DB, queries []Query) []Finding {
	var out []Finding
	for _, q := range queries {
		if !q.Hot {
			continue
		}
		details, err := sqlitePlan(ctx, db, q.SQL)
		if err != nil {
			out = append(out, Finding{Rule: "QY-4", Object: "query " + q.Name, Message: "EXPLAIN QUERY PLAN failed: " + err.Error()})
			continue
		}
		for _, d := range details {
			if table, bad := fullScan(d); bad {
				out = append(out, Finding{Rule: "QY-4", Object: "query " + q.Name, Message: "full table scan of " + table + " (" + d + "); add or reorder an index (QY-7) or use keyset pagination (QY-5)"})
			}
			if q.List() && q.Ordered && strings.Contains(d, "USE TEMP B-TREE FOR") && strings.Contains(d, "ORDER BY") {
				out = append(out, Finding{Rule: "QY-4", Object: "query " + q.Name, Message: "list query sorts in a temporary b-tree (" + d + "); an index must give the order"})
			}
		}
	}
	return out
}

func sqlitePlan(ctx context.Context, db *sql.DB, statement string) ([]string, error) {
	// The driver wants a value for every marker; NULL leaves the plan as it is
	// for a statement prepared without values.
	args := make([]any, placeholderCount(statement))
	rows, err := db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only query
	var out []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			return nil, err
		}
		out = append(out, detail)
	}
	return out, rows.Err()
}

// fullScan reports a "SCAN <table>" plan line that does not use an index.
func fullScan(detail string) (string, bool) {
	m := sqliteScanRE.FindStringSubmatch(detail)
	if m == nil {
		return "", false
	}
	switch m[1] {
	case "SUBQUERY", "CONSTANT":
		return "", false
	}
	if strings.Contains(m[2], "USING") {
		return "", false
	}
	return m[1], true
}
