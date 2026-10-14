// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/storetest/dbrules"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/migration"
)

const binaryVersion = "1.0.0"

func TestMain(m *testing.M) { runWithPostgres(m) }

func open(t *testing.T, dsn string, mutate ...func(*postgres.Options)) *postgres.Store {
	t.Helper()
	opts := postgres.Options{DSN: dsn, BinaryVersion: binaryVersion}
	for _, f := range mutate {
		f(&opts)
	}
	s, err := postgres.Open(t.Context(), opts)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	return s
}

func connect(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.WithoutCancel(t.Context())) })
	return conn
}

func embedded(t *testing.T) []migration.Migration {
	t.Helper()
	fsys, dir := postgres.Migrations()
	m, err := migration.Load(fsys, dir)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func readApplied(t *testing.T, conn *pgx.Conn) []migration.Applied {
	t.Helper()
	rows, err := conn.Query(t.Context(), `SELECT version, checksum, binary_version, compatibility_floor FROM schema_migration ORDER BY version LIMIT 1000`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []migration.Applied
	for rows.Next() {
		var a migration.Applied
		if err := rows.Scan(&a.Version, &a.Checksum, &a.BinaryVersion, &a.Floor); err != nil {
			t.Fatal(err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestMigrations(t *testing.T) {
	dsn := newPostgresDatabase(t)
	fake := clock.NewFake(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))
	s := open(t, dsn, func(o *postgres.Options) { o.Clock = fake })
	want := embedded(t)
	conn := connect(t, dsn)
	got := readApplied(t, conn)
	if len(got) != len(want) || len(want) < 2 {
		t.Fatalf("applied %d migrations, embedded %d", len(got), len(want))
	}
	for i, m := range want {
		a := got[i]
		if a.Version != m.Version || a.Checksum != m.Checksum || a.BinaryVersion != binaryVersion || a.Floor != m.RecordedFloor(binaryVersion) {
			t.Errorf("row %d = %+v, want version %d checksum %s binary %s floor %s", i, a, m.Version, m.Checksum, binaryVersion, m.RecordedFloor(binaryVersion))
		}
	}
	var same int
	if err := conn.QueryRow(t.Context(), `SELECT count(*) FROM schema_migration WHERE created_at = applied_at AND applied_at = $1`, fake.Now()).Scan(&same); err != nil || same != len(want) {
		t.Fatalf("created_at must equal applied_at = the injected instant: %d rows, %v", same, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// A restart applies nothing.
	open(t, dsn)
	if again := readApplied(t, conn); len(again) != len(want) {
		t.Fatalf("restart changed the history: %d rows", len(again))
	}
}

func TestConcurrentStartsApplyEachMigrationOnce(t *testing.T) {
	dsn := newPostgresDatabase(t)
	errs := make(chan error, 4)
	for range cap(errs) {
		go func() {
			s, err := postgres.Open(t.Context(), postgres.Options{DSN: dsn, BinaryVersion: binaryVersion})
			if err == nil {
				err = s.Close()
			}
			errs <- err
		}()
	}
	for range cap(errs) {
		if err := <-errs; err != nil {
			t.Errorf("start: %v", err)
		}
	}
	if got := readApplied(t, connect(t, dsn)); len(got) != len(embedded(t)) {
		t.Fatalf("history has %d rows", len(got))
	}
}

func TestRefuseNewerSchema(t *testing.T) {
	dsn := newPostgresDatabase(t)
	s := open(t, dsn)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	next := len(embedded(t)) + 1
	conn := connect(t, dsn)
	if _, err := conn.Exec(t.Context(), `INSERT INTO schema_migration (version, checksum, applied_at, binary_version, compatibility_floor, created_at) VALUES ($1, $2, now(), '2.0.0', '1.5.0', now())`,
		next, strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	_, err := postgres.Open(t.Context(), postgres.Options{DSN: dsn, BinaryVersion: binaryVersion})
	var fe *migration.FloorError
	if !errors.Is(err, migration.ErrBinaryTooOld) || !errors.As(err, &fe) {
		t.Fatalf("open error %v", err)
	}
	if fe.BinaryVersion != binaryVersion || fe.SchemaVersion != next || fe.Required != "1.5.0" {
		t.Fatalf("floor error %+v", fe)
	}
	for _, want := range []string{binaryVersion, fmt.Sprintf("%04d", next), "1.5.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not name %q", err, want)
		}
	}
	// At the floor, an older binary than the schema starts (rolling upgrade).
	if _, err := conn.Exec(t.Context(), `UPDATE schema_migration SET compatibility_floor = '1.0.0' WHERE version = $1`, next); err != nil {
		t.Fatal(err)
	}
	open(t, dsn)
}

func TestChecksumMismatch(t *testing.T) {
	dsn := newPostgresDatabase(t)
	s := open(t, dsn)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	conn := connect(t, dsn)
	if _, err := conn.Exec(t.Context(), `UPDATE schema_migration SET checksum = $1 WHERE version = 1`, strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}
	_, err := postgres.Open(t.Context(), postgres.Options{DSN: dsn, BinaryVersion: binaryVersion})
	var ce *migration.ChecksumError
	if !errors.Is(err, migration.ErrChecksumMismatch) || !errors.As(err, &ce) || ce.Version != 1 {
		t.Fatalf("open error %v", err)
	}
}

func TestLintSchema(t *testing.T) {
	dsn := newPostgresDatabase(t)
	open(t, dsn)
	policy, err := dbrules.LoadPolicy()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range dbrules.LintPostgres(t.Context(), connect(t, dsn), policy) {
		t.Errorf("%s", f)
	}
}

func TestHotQueryPlans(t *testing.T) {
	dsn := newPostgresDatabase(t)
	open(t, dsn)
	queries, err := dbrules.LoadQueries(os.DirFS("."), "queries", dbrules.DialectPostgres)
	if err != nil {
		t.Fatal(err)
	}
	hot := 0
	for _, q := range queries {
		if q.Hot {
			hot++
		}
	}
	if hot == 0 {
		t.Fatal("no query is tagged -- orcon:hot, so the plan check would pass without checking anything")
	}
	conn, notices, err := dbrules.ConnectForPlans(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.WithoutCancel(t.Context())) //nolint:errcheck // test connection
	for _, f := range dbrules.CheckPlansPostgres(t.Context(), conn, queries, engineCalls(), notices) {
		t.Errorf("%s", f)
	}
}

// postgresEngine runs the upgrade harness of dbrules on the real adapter: the
// real migration runner against a fresh database per Fresh.
type postgresEngine struct {
	t    *testing.T
	conn *pgx.Conn
}

func (e *postgresEngine) Name() string { return dbrules.DialectPostgres }

func (e *postgresEngine) Fresh(testing.TB) error {
	if e.conn != nil {
		if err := e.conn.Close(context.WithoutCancel(e.t.Context())); err != nil {
			return err
		}
	}
	conn, err := pgx.Connect(e.t.Context(), newPostgresDatabase(e.t))
	if err != nil {
		return err
	}
	e.t.Cleanup(func() { _ = conn.Close(context.WithoutCancel(e.t.Context())) })
	e.conn = conn
	return nil
}

func (e *postgresEngine) Exec(ctx context.Context, sqlText string) error {
	_, err := e.conn.Exec(ctx, sqlText, pgx.QueryExecModeSimpleProtocol)
	return err
}

func (e *postgresEngine) Query(ctx context.Context, statement string) ([][]string, error) {
	rows, err := e.conn.Query(ctx, statement, pgx.QueryExecModeSimpleProtocol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make([]string, len(values))
		for i, v := range values {
			row[i] = fmt.Sprint(v)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (e *postgresEngine) OrganizationTables(ctx context.Context) ([]string, error) {
	rows, err := e.conn.Query(ctx, `SELECT table_name FROM information_schema.columns WHERE table_schema = 'public' AND column_name = 'org_id' ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (e *postgresEngine) CheckIntegrity(ctx context.Context) []dbrules.Finding {
	return dbrules.PostgresIntegrity(ctx, e.conn)
}

func TestUpgradeFromSeededPreviousVersion(t *testing.T) {
	requirePostgres(t)
	embeddedFS, dir := postgres.Migrations()
	migrations, err := fs.Sub(embeddedFS, dir)
	if err != nil {
		t.Fatal(err)
	}
	testdata, err := fs.Sub(os.DirFS("."), "migrations/testdata")
	if err != nil {
		t.Fatal(err)
	}
	e := &postgresEngine{t: t}
	apply := func(ctx context.Context, upTo int) error {
		return postgres.Migrate(ctx, e.conn, postgres.MigrateOptions{BinaryVersion: binaryVersion, UpTo: upTo})
	}
	dbrules.RunUpgradeTests(t, e, migrations, testdata, apply)
}

// engineCalls runs every function of the engine plane with literal arguments, so
// the plans of the statements inside their SECURITY DEFINER bodies are checked
// through auto_explain (orcon-db CK-11): no full table scan, no sort in a list.
func engineCalls() []dbrules.EngineCall {
	const worker = "'00000000-0000-0000-0000-000000000001'::uuid"
	return []dbrules.EngineCall{
		{Name: "claim_tokens", List: true, SQL: "SELECT org_id FROM orcon_engine.claim_tokens(" + worker + ", NULL, 10, 1000, now(), 3)"},
		{Name: "claim_tokens with fairness keys", List: true, SQL: "SELECT org_id FROM orcon_engine.claim_tokens(" + worker + ", ARRAY['k'], 10, 1000, now(), 3)"},
		{Name: "renew_leases", SQL: "SELECT org_id FROM orcon_engine.renew_leases(" + worker + ", 1000, now())"},
		{Name: "claim_last_chance_token", List: true, SQL: "SELECT org_id FROM orcon_engine.claim_last_chance_token(" + worker + ", 1000, now(), 3)"},
		{Name: "oldest_last_chance", SQL: "SELECT orcon_engine.oldest_last_chance(3)"},
		{Name: "release_expired_leases", SQL: "SELECT org_id FROM orcon_engine.release_expired_leases(10, 3, now())"},
		{Name: "release_worker_leases", SQL: "SELECT org_id FROM orcon_engine.release_worker_leases(" + worker + ", 3, now(), FALSE)"},
		{Name: "release_token_lease", SQL: "SELECT org_id FROM orcon_engine.release_token_lease(" + worker + ", 1, " + worker + ", 3, now())"},
		{Name: "appended_events", List: true, SQL: "SELECT org_id FROM orcon_engine.appended_events()"},
		{Name: "enqueue_outbox", SQL: "SELECT orcon_engine.enqueue_outbox(" + worker + ", " + worker + ", 't', 'x'::bytea, now())"},
		{Name: "fire_due_timers", List: true, SQL: "SELECT org_id FROM orcon_engine.fire_due_timers(10, now())"},
		{Name: "claim_outbox", List: true, SQL: "SELECT org_id FROM orcon_engine.claim_outbox(" + worker + ", 10, 1000, now())"},
		{Name: "resolve_wait_secret", SQL: "SELECT org_id FROM orcon_engine.resolve_wait_secret(" + worker + ")"},
	}
}
