// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"
)

func upgradeFS(t *testing.T, kind, engine string) (migrations, testdata fs.FS) {
	t.Helper()
	base := "testdata/upgrade/" + kind + "/" + engine
	m, err := fs.Sub(os.DirFS(base), "migrations")
	if err != nil {
		t.Fatal(err)
	}
	d, err := fs.Sub(os.DirFS(base), "testdata")
	if err != nil {
		t.Fatal(err)
	}
	return m, d
}

// engines returns the reference engines available in this run.
func engines(t *testing.T) []Engine {
	t.Helper()
	out := []Engine{&sqliteEngine{}}
	if pgServer.enabled {
		out = append(out, &postgresEngine{t: t})
	}
	return out
}

// upgradeResult runs the upgrade to the newest migration of a fixture.
func upgradeResult(t *testing.T, e Engine, kind string) []Finding {
	t.Helper()
	m, d := upgradeFS(t, kind, e.Name())
	numbers, err := migrationNumbers(m)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UpgradeFindings(t.Context(), t, e, numbers[len(numbers)-1], m, d, referenceApplier(e, m))
	if err != nil {
		t.Fatalf("the harness failed: %v", err)
	}
	return got
}

// TestUpgradeFromSeededPreviousVersion runs the good pair through the harness
// on every available engine.
func TestUpgradeFromSeededPreviousVersion(t *testing.T) {
	t.Run("sqlite", func(t *testing.T) {
		e := &sqliteEngine{}
		m, d := upgradeFS(t, "good", e.Name())
		RunUpgradeTests(t, e, m, d, referenceApplier(e, m))
	})
	t.Run("postgres", func(t *testing.T) {
		requirePostgres(t)
		e := &postgresEngine{t: t}
		m, d := upgradeFS(t, "good", e.Name())
		RunUpgradeTests(t, e, m, d, referenceApplier(e, m))
	})
}

// TestUpgradeDetectsAMigrationThatBreaksSeededData plants a migration that
// destroys the seeded rows: its verify file must return them.
func TestUpgradeDetectsAMigrationThatBreaksSeededData(t *testing.T) {
	for _, e := range engines(t) {
		t.Run(e.Name(), func(t *testing.T) {
			got := upgradeResult(t, e, "bad")
			if len(got) == 0 || !strings.Contains(describe(got), "name lost for thing") {
				t.Fatalf("the broken migration was not detected:\n%s", describe(got))
			}
		})
	}
}

func TestUpgradeGoodPairHasNoFindings(t *testing.T) {
	for _, e := range engines(t) {
		t.Run(e.Name(), func(t *testing.T) {
			if got := upgradeResult(t, e, "good"); len(got) != 0 {
				t.Fatalf("findings:\n%s", describe(got))
			}
		})
	}
}

func TestUpgradeRequiresTwoOrganizationsInSeeds(t *testing.T) {
	for _, e := range engines(t) {
		t.Run(e.Name(), func(t *testing.T) {
			got := upgradeResult(t, e, "one-organization")
			if !strings.Contains(describe(got), "rows of two organizations") {
				t.Fatalf("a seed with one organization was accepted:\n%s", describe(got))
			}
		})
	}
}

func TestUpgradeReportsMissingFiles(t *testing.T) {
	e := &sqliteEngine{}
	m, _ := upgradeFS(t, "good", e.Name())
	empty := os.DirFS(t.TempDir())
	got, err := UpgradeFindings(t.Context(), t, e, 2, m, empty, referenceApplier(e, m))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(describe(got), "missing 0001_seed.sql") {
		t.Fatalf("a missing seed file was not reported:\n%s", describe(got))
	}
	got, err = UpgradeFindings(t.Context(), t, e, 1, m, empty, referenceApplier(e, m))
	if err != nil || !strings.Contains(describe(got), "missing 0001_verify.sql") {
		t.Fatalf("a missing verify file was not reported: %v\n%s", err, describe(got))
	}
}

func TestUpgradeHarnessErrors(t *testing.T) {
	e := &sqliteEngine{}
	if _, err := migrationNumbers(os.DirFS(t.TempDir())); err == nil {
		t.Error("no migrations must be an error")
	}
	m, d := upgradeFS(t, "good", e.Name())
	failing := func(_ context.Context, upTo int) error { return errors.New("runner failed at " + strconv.Itoa(upTo)) }
	if _, err := UpgradeFindings(t.Context(), t, e, 1, m, d, failing); err == nil || !strings.Contains(err.Error(), "runner failed at 1") {
		t.Errorf("a failing runner must be an error, got %v", err)
	}
}

func TestMigrationNumbersMustBeContiguous(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"0001_a.sql", "0003_c.sql"} {
		if err := os.WriteFile(dir+"/"+n, []byte("SELECT 1;"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := migrationNumbers(os.DirFS(dir)); err == nil || !strings.Contains(err.Error(), "contiguous") {
		t.Fatalf("got %v", err)
	}
}

func TestSplitStatements(t *testing.T) {
	got, err := splitStatements("SELECT 1; -- ; not a split\nSELECT 'a;b' ;\n/* ; */ SELECT $$ ; $$;\n;\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"SELECT 1", "-- ; not a split\nSELECT 'a;b'", "/* ; */ SELECT $$ ; $$"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("statement %d = %q, want %q", i, got[i], want[i])
		}
	}
}
