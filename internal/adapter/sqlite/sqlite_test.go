// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/adapter/storetest/dbrules"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/migration"
)

const binaryVersion = "1.0.0"

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// TestHelperHoldLock is the body of the helper process of TestSingleProcessLock:
// it opens the store, announces it and waits until its input closes. Run
// normally it does nothing.
func TestHelperHoldLock(t *testing.T) {
	path := os.Getenv("ORCON_SQLITE_HOLD_LOCK")
	if path == "" {
		t.Skip("helper of TestSingleProcessLock")
	}
	s, err := sqlite.Open(t.Context(), sqlite.Options{Path: path, BinaryVersion: binaryVersion})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := fmt.Fprintln(os.Stdout, "locked"); err != nil {
		t.Fatal(err)
	}
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func open(t *testing.T, path string, mutate ...func(*sqlite.Options)) *sqlite.Store {
	t.Helper()
	opts := sqlite.Options{Path: path, BinaryVersion: binaryVersion, Readers: 3}
	for _, f := range mutate {
		f(&opts)
	}
	s, err := sqlite.Open(t.Context(), opts)
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

func dbPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "orcon.db")
}

// rawDB opens the file without the adapter, for tests that edit schema_migration.
func rawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func embeddedMigrations(t *testing.T) []migration.Migration {
	t.Helper()
	fsys, dir := sqlite.Migrations()
	m, err := migration.Load(fsys, dir)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMigrations(t *testing.T) {
	path := dbPath(t)
	fake := clock.NewFake(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))
	s := open(t, path, func(o *sqlite.Options) { o.Clock = fake })
	want := embeddedMigrations(t)
	read := func(db *sql.DB) []migration.Applied {
		rows, err := db.QueryContext(t.Context(), `SELECT version, checksum, binary_version, compatibility_floor FROM schema_migration ORDER BY version LIMIT 1000`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close() //nolint:errcheck // read-only query
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
	got := read(s.Reader())
	if len(got) != len(want) || len(want) < 2 {
		t.Fatalf("applied %d migrations, embedded %d", len(got), len(want))
	}
	for i, m := range want {
		a := got[i]
		if a.Version != m.Version || a.Checksum != m.Checksum || a.BinaryVersion != binaryVersion {
			t.Errorf("row %d = %+v, want version %d checksum %s binary %s", i, a, m.Version, m.Checksum, binaryVersion)
		}
		if a.Floor != m.RecordedFloor(binaryVersion) {
			t.Errorf("migration %d floor %q, want %q", m.Version, a.Floor, m.RecordedFloor(binaryVersion))
		}
	}
	var same int
	if err := s.Reader().QueryRowContext(t.Context(), `SELECT count(*) FROM schema_migration WHERE created_at = applied_at AND applied_at = ?1`, fake.Now().UnixMicro()).Scan(&same); err != nil || same != len(want) {
		t.Fatalf("created_at must equal applied_at = the injected instant: %d rows, %v", same, err)
	}
	// A restart applies nothing and changes nothing.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2 := open(t, path, func(o *sqlite.Options) { o.Clock = clock.NewFake(fake.Now().Add(time.Hour)) })
	if again := read(s2.Reader()); len(again) != len(want) {
		t.Fatalf("restart changed the history: %d rows", len(again))
	}
}

func TestMigrationsRecordTheDeclaredFloor(t *testing.T) {
	fsys := fstestFS(map[string]string{
		"0001_first.sql":  header("self") + "CREATE TABLE schema_migration (version INTEGER PRIMARY KEY, checksum TEXT NOT NULL, applied_at INTEGER NOT NULL, binary_version TEXT NOT NULL, compatibility_floor TEXT NOT NULL, created_at INTEGER NOT NULL) STRICT;\n",
		"0002_second.sql": header("0.9.0") + "CREATE TABLE other (id INTEGER PRIMARY KEY) STRICT;\n",
	})
	s := open(t, dbPath(t), func(o *sqlite.Options) { o.Migrations, o.MigrationsDir = fsys, "." })
	rows, err := s.Reader().QueryContext(t.Context(), `SELECT compatibility_floor FROM schema_migration ORDER BY version LIMIT 10`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // read-only query
	var floors []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			t.Fatal(err)
		}
		floors = append(floors, f)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(floors, ",") != binaryVersion+",0.9.0" {
		t.Fatalf("floors %v: self must record the binary version and a declared floor itself", floors)
	}
}

func TestRefuseNewerSchema(t *testing.T) {
	path := dbPath(t)
	s := open(t, path)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	next := len(embeddedMigrations(t)) + 1
	db := rawDB(t, path)
	insert := func(version int, floor string) {
		t.Helper()
		if _, err := db.ExecContext(t.Context(), `INSERT INTO schema_migration (version, checksum, applied_at, binary_version, compatibility_floor, created_at) VALUES (?1, ?2, 1, '2.0.0', ?3, 1)`,
			version, strings.Repeat("0", 64), floor); err != nil {
			t.Fatal(err)
		}
	}
	// A newer migration whose floor is above this binary: refuse, naming three versions.
	insert(next, "1.5.0")
	_, err := sqlite.Open(t.Context(), sqlite.Options{Path: path, BinaryVersion: binaryVersion})
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
	// A refusal leaves the file usable: lowering the floor to this binary's version lets it start.
	if _, err := db.ExecContext(t.Context(), `UPDATE schema_migration SET compatibility_floor = '1.0.0' WHERE version = ?1`, next); err != nil {
		t.Fatal(err)
	}
	s = open(t, path)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestChecksumMismatch(t *testing.T) {
	path := dbPath(t)
	s := open(t, path)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	db := rawDB(t, path)
	if _, err := db.ExecContext(t.Context(), `UPDATE schema_migration SET checksum = ?1 WHERE version = 1`, strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}
	_, err := sqlite.Open(t.Context(), sqlite.Options{Path: path, BinaryVersion: binaryVersion})
	var ce *migration.ChecksumError
	if !errors.Is(err, migration.ErrChecksumMismatch) || !errors.As(err, &ce) || ce.Version != 1 {
		t.Fatalf("open error %v", err)
	}
}

func TestFailedMigrationLeavesNothingBehind(t *testing.T) {
	fsys := fstestFS(map[string]string{
		"0001_first.sql":  header("self") + "CREATE TABLE schema_migration (version INTEGER PRIMARY KEY, checksum TEXT NOT NULL, applied_at INTEGER NOT NULL, binary_version TEXT NOT NULL, compatibility_floor TEXT NOT NULL, created_at INTEGER NOT NULL) STRICT;\n",
		"0002_second.sql": header("self") + "CREATE TABLE half (id INTEGER PRIMARY KEY) STRICT;\nINSERT INTO missing_table VALUES (1);\n",
	})
	path := dbPath(t)
	_, err := sqlite.Open(t.Context(), sqlite.Options{Path: path, BinaryVersion: binaryVersion, Migrations: fsys, MigrationsDir: "."})
	if err == nil {
		t.Fatal("a failing migration was accepted")
	}
	db := rawDB(t, path)
	var n int
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM sqlite_schema WHERE name IN ('half')`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("the failed migration left a table behind: %d, %v", n, err)
	}
	var applied int
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM schema_migration`).Scan(&applied); err != nil || applied != 1 {
		t.Fatalf("history after the failure: %d rows, %v", applied, err)
	}
}

func TestSingleProcessLock(t *testing.T) {
	path := dbPath(t)
	first := open(t, path)
	// A second opener in this process is refused while the first holds the file.
	if _, err := sqlite.Open(t.Context(), sqlite.Options{Path: path, BinaryVersion: binaryVersion}); !errors.Is(err, sqlite.ErrLocked) {
		t.Fatalf("second open error %v, want ErrLocked", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := open(t, path)
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}

	// A second process is refused too, and the lock dies with its holder.
	helper := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperHoldLock$") //nolint:gosec // re-executes this test binary
	helper.Env = append(os.Environ(), "ORCON_SQLITE_HOLD_LOCK="+path)
	stdin, err := helper.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	killed := false
	t.Cleanup(func() {
		if !killed {
			_ = helper.Process.Kill()
			_ = helper.Wait()
		}
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "locked" {
		t.Fatalf("helper said %q, %v", line, err)
	}
	_, err = sqlite.Open(t.Context(), sqlite.Options{Path: path, BinaryVersion: binaryVersion})
	if !errors.Is(err, sqlite.ErrLocked) {
		t.Fatalf("open while another process holds the file: %v", err)
	}
	if !strings.Contains(err.Error(), "process") {
		t.Logf("message does not name the holder: %v", err)
	}
	_ = stdin
	if err := helper.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = helper.Wait()
	killed = true
	s := open(t, path)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestForeignKeysOnEveryConnection(t *testing.T) {
	s := open(t, dbPath(t), func(o *sqlite.Options) { o.Readers = 4 })
	ctx := t.Context()
	// Hold every reader and the writer at the same time: successive queries would reuse one connection.
	var readers []*sql.Conn
	for range 4 {
		c, err := s.Reader().Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		readers = append(readers, c)
	}
	writer, err := s.Writer().Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range readers {
		for _, f := range dbrules.CheckSQLiteConnection(ctx, c, dbrules.SQLiteReader) {
			t.Errorf("reader %d: %s", i, f)
		}
	}
	for _, f := range dbrules.CheckSQLiteConnection(ctx, writer, dbrules.SQLiteWriter) {
		t.Errorf("writer: %s", f)
	}
	for _, c := range readers {
		_ = c.Close()
	}
	_ = writer.Close()
}

func TestWeakerSynchronousIsAnExplicitOptIn(t *testing.T) {
	s := open(t, dbPath(t), func(o *sqlite.Options) { o.Synchronous = sqlite.SynchronousNormal })
	conn, err := s.Writer().Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() //nolint:errcheck // test connection
	if got := dbrules.CheckSQLiteConnection(t.Context(), conn, dbrules.SQLiteWriter); len(got) == 0 {
		t.Fatal("NORMAL must be reported unless allowed")
	}
	for _, f := range dbrules.CheckSQLiteConnection(t.Context(), conn, dbrules.SQLiteWriter, dbrules.AllowSynchronous(int(sqlite.SynchronousNormal))) {
		t.Errorf("%s", f)
	}
}

func TestLintSchema(t *testing.T) {
	s := open(t, dbPath(t))
	policy, err := dbrules.LoadPolicy()
	if err != nil {
		t.Fatal(err)
	}
	findings := dbrules.LintSQLite(t.Context(), s.Reader(), policy)
	// The trigger registry (SPEC-002 T1, criterion E-DB-1) reports under its own name.
	t.Run("trigger", func(t *testing.T) {
		for _, f := range findings {
			if isTriggerObject(f) {
				t.Errorf("%s", f)
			}
		}
	})
	t.Run("other", func(t *testing.T) {
		for _, f := range findings {
			if !isTriggerObject(f) {
				t.Errorf("%s", f)
			}
		}
	})
}

// isTriggerObject reports whether a schema finding is about the trigger
// registry: its tables and the activation state of the flow.
func isTriggerObject(f dbrules.Finding) bool {
	return strings.Contains(f.Object, "trigger") || strings.Contains(f.Object, "activation_state")
}

func TestHotQueryPlans(t *testing.T) {
	s := open(t, dbPath(t))
	queries, err := dbrules.LoadQueries(os.DirFS("."), "queries", dbrules.DialectSQLite)
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
	for _, f := range dbrules.CheckPlansSQLite(t.Context(), s.Reader(), queries) {
		t.Errorf("%s", f)
	}
}

// sqliteEngine runs the upgrade harness of dbrules on the real adapter: the
// real connection parameters and the real migration runner.
type sqliteEngine struct {
	db *sql.DB
}

func (e *sqliteEngine) Name() string { return dbrules.DialectSQLite }

func (e *sqliteEngine) Fresh(tb testing.TB) error {
	tb.Helper()
	if e.db != nil {
		if err := e.db.Close(); err != nil {
			return err
		}
	}
	db, err := sql.Open("sqlite", sqlite.DSN(filepath.Join(tb.TempDir(), "upgrade.db"), sqlite.RoleWriter, sqlite.Options{}))
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	tb.Cleanup(func() { _ = db.Close() })
	e.db = db
	return nil
}

func (e *sqliteEngine) Exec(ctx context.Context, sqlText string) error {
	//orcon:dynamic-sql test helper: runs the statements of the rule checks
	_, err := e.db.ExecContext(ctx, sqlText)
	return err
}

func (e *sqliteEngine) Query(ctx context.Context, statement string) ([][]string, error) {
	//orcon:dynamic-sql test helper: runs the statements of the rule checks
	rows, err := e.db.QueryContext(ctx, statement)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only query
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out [][]string
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]string, len(cols))
		for i, v := range values {
			row[i] = fmt.Sprint(v)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (e *sqliteEngine) OrganizationTables(ctx context.Context) ([]string, error) {
	rows, err := e.db.QueryContext(ctx, `SELECT m.name FROM sqlite_schema m, pragma_table_info(m.name) c WHERE m.type = 'table' AND c.name = 'org_id' ORDER BY m.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only query
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

func (e *sqliteEngine) CheckIntegrity(ctx context.Context) []dbrules.Finding {
	return dbrules.SQLiteIntegrity(ctx, e.db)
}

func TestUpgradeFromSeededPreviousVersion(t *testing.T) {
	embedded, dir := sqlite.Migrations()
	migrations, err := fs.Sub(embedded, dir)
	if err != nil {
		t.Fatal(err)
	}
	testdata, err := fs.Sub(os.DirFS("."), "migrations/testdata")
	if err != nil {
		t.Fatal(err)
	}
	e := &sqliteEngine{}
	apply := func(ctx context.Context, upTo int) error {
		return sqlite.Migrate(ctx, e.db, sqlite.MigrateOptions{BinaryVersion: binaryVersion, UpTo: upTo})
	}
	dbrules.RunUpgradeTests(t, e, migrations, testdata, apply)
}
