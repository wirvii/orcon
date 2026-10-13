// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

// applicationRole is the runtime role of the application (NM-13, TI-5).
const applicationRole = "orcon_application"

// engineSchemas are the only schemas that may hold functions (NM-13).
var engineSchemas = map[string]bool{"orcon_engine": true, "orcon_context": true}

// policyTemplate is the row-level security predicate of TI-4. %s is the column
// that holds the organization (org_id, or id on the org table).
const policyTemplate = `%[1]s = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR %[1]s = ANY ((SELECT orcon_context.current_org_ids()))`

// LintPostgres checks the schema of a PostgreSQL database against the rulebook
// by reading its catalog (pg_class, pg_attribute, pg_constraint, pg_index,
// pg_policy, pg_type, pg_proc, pg_views, pg_trigger, pg_namespace, pg_roles and
// has_table_privilege). The migrations must be applied first. The connection
// needs to read the catalog and to create a temporary table, which it does only
// inside a transaction that is rolled back. A catalog that cannot be read is a
// finding, not a pass.
func LintPostgres(ctx context.Context, conn *pgx.Conn, p *Policy) []Finding {
	m, err := readPostgresModel(ctx, conn, p)
	if err != nil {
		return []Finding{{Rule: "CK-4", Object: "catalog", Message: "cannot read the PostgreSQL catalog: " + err.Error()}}
	}
	return lintModel(m, p)
}

type pgTable struct {
	oid        uint32
	t          *table
	schema     string
	owner      string
	rlsOn      bool
	rlsForced  bool
	policies   map[string]pgPolicy
	canUpdate  bool
	canDelete  bool
	privsKnown bool
}

type pgPolicy struct {
	command   string
	using     string
	withCheck string
}

func readPostgresModel(ctx context.Context, conn *pgx.Conn, p *Policy) (*model, error) {
	m := &model{dialect: DialectPostgres}
	tables, err := pgTables(ctx, conn)
	if err != nil {
		return nil, err
	}
	oids := make([]uint32, 0, len(tables))
	byOID := map[uint32]*pgTable{}
	for _, t := range tables {
		oids = append(oids, t.oid)
		byOID[t.oid] = t
		m.tables = append(m.tables, t.t)
	}
	steps := []func() error{
		func() error { return pgColumns(ctx, conn, oids, byOID) },
		func() error { return pgConstraints(ctx, conn, oids, byOID) },
		func() error { return pgIndexes(ctx, conn, oids, byOID) },
		func() error { return pgPolicies(ctx, conn, oids, byOID) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}
	role, err := pgApplicationRole(ctx, conn)
	if err != nil {
		return nil, err
	}
	if role.exists {
		if err := pgPrivileges(ctx, conn, tables); err != nil {
			return nil, err
		}
	}
	extra, others, err := pgObjects(ctx, conn, p)
	if err != nil {
		return nil, err
	}
	m.extra = append(m.extra, extra...)
	m.otherNames = others
	m.extra = append(m.extra, pgTableFindings(ctx, conn, tables, p)...)
	m.extra = append(m.extra, pgRoleFindings(role, tables)...)
	return m, nil
}

func pgTables(ctx context.Context, conn *pgx.Conn) ([]*pgTable, error) {
	rows, err := conn.Query(ctx, `
SELECT c.oid, c.relname, n.nspname, r.rolname, c.relrowsecurity, c.relforcerowsecurity
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_roles r ON r.oid = c.relowner
WHERE c.relkind IN ('r', 'p')
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
  AND n.nspname NOT LIKE 'pg\_toast%' AND n.nspname NOT LIKE 'pg\_temp%'
ORDER BY n.nspname, c.relname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*pgTable
	for rows.Next() {
		t := &pgTable{t: &table{}, policies: map[string]pgPolicy{}}
		if err := rows.Scan(&t.oid, &t.t.name, &t.schema, &t.owner, &t.rlsOn, &t.rlsForced); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func pgColumns(ctx context.Context, conn *pgx.Conn, oids []uint32, byOID map[uint32]*pgTable) error {
	rows, err := conn.Query(ctx, `
SELECT attrelid, attname, format_type(atttypid, atttypmod), attnotnull
FROM pg_attribute
WHERE attnum > 0 AND NOT attisdropped AND attrelid = ANY($1)
ORDER BY attrelid, attnum`, oids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var oid uint32
		var c column
		if err := rows.Scan(&oid, &c.name, &c.typ, &c.notNull); err != nil {
			return err
		}
		byOID[oid].t.columns = append(byOID[oid].t.columns, &c)
	}
	return rows.Err()
}

func pgConstraints(ctx context.Context, conn *pgx.Conn, oids []uint32, byOID map[uint32]*pgTable) error {
	rows, err := conn.Query(ctx, `
SELECT con.conrelid, con.conname, con.contype::text, con.confdeltype::text,
  coalesce(ARRAY(SELECT a.attname::text FROM unnest(con.conkey) WITH ORDINALITY k(num, ord)
                 JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.num ORDER BY k.ord), '{}'),
  coalesce((SELECT relname::text FROM pg_class WHERE oid = con.confrelid), ''),
  coalesce(ARRAY(SELECT a.attname::text FROM unnest(con.confkey) WITH ORDINALITY k(num, ord)
                 JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.num ORDER BY k.ord), '{}'),
  CASE WHEN con.contype = 'c' THEN pg_get_constraintdef(con.oid) ELSE '' END
FROM pg_constraint con
WHERE con.conrelid = ANY($1) AND con.contype IN ('p', 'u', 'f', 'c')
ORDER BY con.conrelid, con.conname`, oids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var oid uint32
		var name, kind, delType, refTable, def string
		var cols, refCols []string
		if err := rows.Scan(&oid, &name, &kind, &delType, &cols, &refTable, &refCols, &def); err != nil {
			return err
		}
		t := byOID[oid].t
		switch kind {
		case "p":
			t.pkName, t.pkCols = name, cols
		case "f":
			t.fks = append(t.fks, &foreignKey{name: name, cols: cols, refTable: refTable, refCols: refCols, onDelete: pgDeleteAction(delType)})
		case "c":
			toks, err := tokenizePostgres(def)
			if err != nil {
				return fmt.Errorf("constraint %s: %w", name, err)
			}
			t.checks = append(t.checks, &check{name: name, toks: toks})
		}
	}
	return rows.Err()
}

func pgDeleteAction(code string) string {
	switch code {
	case "r":
		return "restrict"
	case "c":
		return "cascade"
	case "n":
		return "set null"
	case "d":
		return "set default"
	}
	return "no action"
}

func pgIndexes(ctx context.Context, conn *pgx.Conn, oids []uint32, byOID map[uint32]*pgTable) error {
	rows, err := conn.Query(ctx, `
SELECT i.indrelid, ic.relname, i.indisunique, i.indisprimary,
  coalesce(pg_get_expr(i.indpred, i.indrelid), ''),
  coalesce(ARRAY(SELECT pg_get_indexdef(i.indexrelid, g, true) FROM generate_series(1, i.indnkeyatts) g ORDER BY g), '{}'),
  i.indexprs IS NOT NULL,
  EXISTS (SELECT 1 FROM pg_constraint con WHERE con.conindid = i.indexrelid AND con.contype = 'u')
FROM pg_index i JOIN pg_class ic ON ic.oid = i.indexrelid
WHERE i.indrelid = ANY($1)
ORDER BY i.indrelid, ic.relname`, oids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var oid uint32
		var ix index
		var primary, constraint bool
		if err := rows.Scan(&oid, &ix.name, &ix.unique, &primary, &ix.predicate, &ix.cols, &ix.hasExpr, &constraint); err != nil {
			return err
		}
		ix.predicate = normalizePredicateText(ix.predicate)
		for i, c := range ix.cols {
			ix.cols[i] = strings.Trim(c, `"`)
		}
		switch {
		case primary:
			ix.kind = "primary"
		case constraint:
			ix.kind = "unique"
		default:
			ix.kind = "index"
		}
		byOID[oid].t.indexes = append(byOID[oid].t.indexes, &ix)
	}
	return rows.Err()
}

func normalizePredicateText(s string) string {
	toks, err := tokenize(s)
	if err != nil {
		return strings.ToLower(s)
	}
	return normalizePredicate(toks)
}

func pgPolicies(ctx context.Context, conn *pgx.Conn, oids []uint32, byOID map[uint32]*pgTable) error {
	rows, err := conn.Query(ctx, `
SELECT polrelid, polname, polcmd::text, coalesce(pg_get_expr(polqual, polrelid), ''), coalesce(pg_get_expr(polwithcheck, polrelid), '')
FROM pg_policy WHERE polrelid = ANY($1)`, oids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var oid uint32
		var name string
		var pol pgPolicy
		if err := rows.Scan(&oid, &name, &pol.command, &pol.using, &pol.withCheck); err != nil {
			return err
		}
		byOID[oid].policies[name] = pol
	}
	return rows.Err()
}

type pgRole struct {
	exists    bool
	superuser bool
	bypassRLS bool
}

func pgApplicationRole(ctx context.Context, conn *pgx.Conn) (pgRole, error) {
	var r pgRole
	err := conn.QueryRow(ctx, `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = $1`, applicationRole).Scan(&r.superuser, &r.bypassRLS)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return r, nil
	case err != nil:
		return r, err
	}
	r.exists = true
	return r, nil
}

func pgPrivileges(ctx context.Context, conn *pgx.Conn, tables []*pgTable) error {
	for _, t := range tables {
		err := conn.QueryRow(ctx, `SELECT has_table_privilege($1, $2::oid, 'UPDATE'), has_table_privilege($1, $2::oid, 'DELETE')`, applicationRole, t.oid).Scan(&t.canUpdate, &t.canDelete)
		if err != nil {
			return err
		}
		t.privsKnown = true
	}
	return nil
}

// pgObjects reads the objects that are not tables: schemas, roles, functions,
// views, triggers and types. It returns the findings about them and the names
// that NM-1, NM-3 and NM-12 apply to.
func pgObjects(ctx context.Context, conn *pgx.Conn, p *Policy) ([]Finding, [][2]string, error) {
	var findings []Finding
	var names [][2]string
	queries := []struct {
		query string
		each  func(a, b string)
	}{
		{`SELECT nspname, '' FROM pg_namespace WHERE nspname NOT LIKE 'pg\_%' AND nspname NOT IN ('information_schema', 'public')`, func(a, _ string) {
			names = append(names, [2]string{"schema", a})
			if !engineSchemas[a] {
				findings = append(findings, Finding{Rule: "NM-13", Object: "schema " + a, Message: "tables live in public; functions only in orcon_engine and orcon_context"})
			}
		}},
		{`SELECT rolname, '' FROM pg_roles WHERE rolname LIKE 'orcon\_%'`, func(a, _ string) {
			names = append(names, [2]string{"role", a})
		}},
		{`SELECT n.nspname, p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname NOT IN ('pg_catalog', 'information_schema') AND n.nspname NOT LIKE 'pg\_toast%'
  AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid = 'pg_proc'::regclass AND d.objid = p.oid AND d.deptype = 'e')`, func(a, b string) {
			names = append(names, [2]string{"function", b})
			if !engineSchemas[a] {
				findings = append(findings, Finding{Rule: "TI-10", Object: "function " + a + "." + b, Message: "functions live only in schemas orcon_engine and orcon_context"})
			}
		}},
		{`SELECT schemaname, viewname FROM pg_views WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
UNION ALL SELECT schemaname, matviewname FROM pg_matviews`, func(a, b string) {
			findings = append(findings, Finding{Rule: "TI-10", Object: "view " + a + "." + b, Message: "views over organization data bypass row-level security in PostgreSQL 14"})
		}},
		{`SELECT t.tgname, c.relname FROM pg_trigger t JOIN pg_class c ON c.oid = t.tgrelid JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE NOT t.tgisinternal AND n.nspname NOT IN ('pg_catalog', 'information_schema')`, func(a, b string) {
			if !p.excepted("TI-10", DialectPostgres, a) {
				findings = append(findings, Finding{Rule: "TI-10", Object: "trigger " + a + " on " + b, Message: "triggers need an owner-approved TI-10 exception in " + fileExceptions})
			}
		}},
		{`SELECT n.nspname, t.typname FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
WHERE n.nspname NOT IN ('pg_catalog', 'information_schema') AND n.nspname NOT LIKE 'pg\_toast%' AND t.typtype = 'e'`, func(a, b string) {
			findings = append(findings, Finding{Rule: "TY-2", Object: "type " + a + "." + b, Message: "ENUM types are forbidden; use text with a CHECK"})
		}},
		{`SELECT n.nspname, t.typname FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
WHERE n.nspname NOT IN ('pg_catalog', 'information_schema') AND n.nspname NOT LIKE 'pg\_toast%' AND t.typtype = 'd'`, func(a, b string) {
			findings = append(findings, Finding{Rule: "TY-2", Object: "type " + a + "." + b, Message: "domains are forbidden"})
		}},
	}
	for _, q := range queries {
		rows, err := conn.Query(ctx, q.query)
		if err != nil {
			return nil, nil, err
		}
		for rows.Next() {
			var a, b string
			if err := rows.Scan(&a, &b); err != nil {
				rows.Close()
				return nil, nil, err
			}
			q.each(a, b)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
	}
	var publicCreate bool
	err := conn.QueryRow(ctx, `
SELECT EXISTS (SELECT 1 FROM pg_namespace n, aclexplode(coalesce(n.nspacl, acldefault('n', n.nspowner))) a
               WHERE n.nspname = 'public' AND a.grantee = 0 AND a.privilege_type = 'CREATE')`).Scan(&publicCreate)
	if err != nil {
		return nil, nil, err
	}
	if publicCreate {
		findings = append(findings, Finding{Rule: "TI-10", Object: "schema public", Message: "CREATE on schema public is revoked from PUBLIC"})
	}
	return findings, names, nil
}

// pgRoleFindings checks the runtime role (TI-5).
func pgRoleFindings(role pgRole, tables []*pgTable) []Finding {
	if !role.exists {
		return []Finding{{Rule: "TI-5", Object: "role " + applicationRole, Message: "the runtime role does not exist"}}
	}
	var out []Finding
	if role.superuser {
		out = append(out, Finding{Rule: "TI-5", Object: "role " + applicationRole, Message: "the runtime role is NOSUPERUSER"})
	}
	if role.bypassRLS {
		out = append(out, Finding{Rule: "TI-5", Object: "role " + applicationRole, Message: "the runtime role is NOBYPASSRLS"})
	}
	for _, t := range tables {
		if t.owner == applicationRole {
			out = append(out, Finding{Rule: "TI-5", Object: "table " + t.t.name, Message: "the runtime role owns nothing"})
		}
	}
	return out
}

var spaceRE = regexp.MustCompile(`\s+`)

func normalizeSpace(s string) string { return strings.TrimSpace(spaceRE.ReplaceAllString(s, " ")) }

// pgTableFindings checks row-level security (TI-4), schema placement (NM-13)
// and the privileges on append-only tables (IN-11).
func pgTableFindings(ctx context.Context, conn *pgx.Conn, tables []*pgTable, p *Policy) []Finding {
	var out []Finding
	expected := map[string][2]string{}
	var probeErr error
	for _, column := range []string{"org_id", "id"} {
		using, check, err := expectedPolicy(ctx, conn, column)
		if err != nil {
			probeErr = err
			break
		}
		expected[column] = [2]string{using, check}
	}
	probeReported := false
	for _, t := range tables {
		obj := "table " + t.t.name
		if t.schema != "public" {
			out = append(out, Finding{Rule: "NM-13", Object: obj, Message: "tables live in schema public (found " + t.schema + ")"})
		}
		if p.appendOnly[t.t.name] && t.privsKnown && (t.canUpdate || t.canDelete) {
			out = append(out, Finding{Rule: "IN-11", Object: obj, Message: "append-only tables grant orcon_application no UPDATE or DELETE"})
		}
		column, policyName := "org_id", "org_isolation"
		switch {
		case t.t.name == "org":
			column, policyName = "id", "org_self_isolation"
		case t.t.column("org_id") == nil:
			continue
		}
		if !t.rlsOn || !t.rlsForced {
			out = append(out, Finding{Rule: "TI-4", Object: obj, Message: "row-level security is ENABLEd and FORCEd (relrowsecurity, relforcerowsecurity)"})
		}
		pol, ok := t.policies[policyName]
		switch {
		case !ok:
			out = append(out, Finding{Rule: "TI-4", Object: obj, Message: "missing policy " + policyName})
		case pol.command != "*":
			out = append(out, Finding{Rule: "TI-4", Object: obj, Message: "policy " + policyName + " is FOR ALL"})
		case probeErr != nil:
			if !probeReported {
				out = append(out, Finding{Rule: "TI-4", Object: "catalog", Message: "cannot build the expected policy text: " + probeErr.Error()})
				probeReported = true
			}
		case normalizeSpace(pol.using) != expected[column][0] || normalizeSpace(pol.withCheck) != expected[column][1]:
			out = append(out, Finding{Rule: "TI-4", Object: obj, Message: "policy " + policyName + " differs from the TI-4 template in USING or WITH CHECK"})
		}
	}
	return out
}

// expectedPolicy returns the normalized USING and WITH CHECK text PostgreSQL
// stores for the TI-4 template on column. It creates the policy on a temporary
// table inside a transaction it rolls back, so the text is right for the
// server version in use.
func expectedPolicy(ctx context.Context, conn *pgx.Conn, column string) (using, withCheck string, err error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) && err == nil {
			err = rbErr
		}
	}()
	predicate := fmt.Sprintf(policyTemplate, column)
	statements := []string{
		`CREATE TEMP TABLE dbrules_policy_probe (org_id uuid, id uuid) ON COMMIT DROP`,
		`ALTER TABLE dbrules_policy_probe ENABLE ROW LEVEL SECURITY`,
		`CREATE POLICY dbrules_probe ON dbrules_policy_probe FOR ALL USING (` + predicate + `) WITH CHECK (` + predicate + `)`,
	}
	for _, s := range statements {
		if _, err := tx.Exec(ctx, s); err != nil {
			return "", "", err
		}
	}
	err = tx.QueryRow(ctx, `SELECT pg_get_expr(polqual, polrelid), pg_get_expr(polwithcheck, polrelid) FROM pg_policy WHERE polname = 'dbrules_probe'`).Scan(&using, &withCheck)
	if err != nil {
		return "", "", err
	}
	return normalizeSpace(using), normalizeSpace(withCheck), nil
}
