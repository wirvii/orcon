// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"
	"time"
)

// TestEngineFunctionsAreOwnedByTheEngineRole checks every orcon_engine function (ADR-0047, orcon-db TI-9):
// owned by orcon_engine, SECURITY DEFINER, not executable by PUBLIC, and executable by the application role
// where it is granted. It then runs prune_batch, which executes as its owner, so a missing privilege of the
// owner (for example UPDATE on an append-only table) fails here.
func TestEngineFunctionsAreOwnedByTheEngineRole(t *testing.T) {
	dsn := newPostgresDatabase(t)
	s := open(t, dsn)
	t.Cleanup(func() { _ = s.Close() })
	conn := connect(t, dsn)
	ctx := t.Context()

	rows, err := conn.Query(ctx, `SELECT p.proname, pg_get_userbyid(p.proowner), p.prosecdef,
		has_function_privilege('public', p.oid, 'EXECUTE'), has_function_privilege('orcon_application', p.oid, 'EXECUTE')
		FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'orcon_engine' ORDER BY p.proname LIMIT 100`)
	if err != nil {
		t.Fatalf("list engine functions: %v", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name, owner string
		var definer, publicExec, appExec bool
		if err := rows.Scan(&name, &owner, &definer, &publicExec, &appExec); err != nil {
			t.Fatalf("scan engine function: %v", err)
		}
		names = append(names, name)
		if owner != "orcon_engine" {
			t.Errorf("orcon_engine.%s is owned by %s, want orcon_engine", name, owner)
		}
		if !definer {
			t.Errorf("orcon_engine.%s is not SECURITY DEFINER", name)
		}
		if publicExec {
			t.Errorf("orcon_engine.%s is executable by PUBLIC", name)
		}
		if name == "prune_batch" && !appExec {
			t.Error("orcon_engine.prune_batch is not executable by orcon_application")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read engine functions: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no orcon_engine functions found")
	}

	var removed int64
	if err := conn.QueryRow(ctx, "SELECT orcon_engine.prune_batch('editor_test_data', 10, $1)", time.Now().UTC()).Scan(&removed); err != nil {
		t.Fatalf("prune_batch through its owner: %v", err)
	}
}
