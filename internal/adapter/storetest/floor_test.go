// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
)

// floorRun migrates fresh databases with the code of this tree, then runs the
// smoke subset of the tree in dir against them as a binary of the given
// version (ADR-0015 section 4, floor verification). The directory dir is the
// checkout of the release that is the compatibility floor of a migration.
func floorRun(t *testing.T, dir, version string) {
	t.Helper()
	env := append(os.Environ(), "ORCON_BINARY_VERSION="+version, "ORCON_TEST_POSTGRES=", "ORCON_FLOOR_SQLITE_PATH=", "ORCON_FLOOR_POSTGRES_DSN=")

	sqlitePath := filepath.Join(t.TempDir(), "floor.db")
	s, err := sqlite.Open(t.Context(), sqlite.Options{Path: sqlitePath, BinaryVersion: binaryVersionDefault})
	if err != nil {
		t.Fatalf("migrate the SQLite database with the current code: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	env = append(env, "ORCON_FLOOR_SQLITE_PATH="+sqlitePath)

	if wantPostgres() && postgresEnabled() {
		dsn := newPostgresDatabase(t)
		p, err := postgres.Open(t.Context(), postgres.Options{DSN: dsn, BinaryVersion: binaryVersionDefault})
		if err != nil {
			t.Fatalf("migrate the PostgreSQL database with the current code: %v", err)
		}
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
		env = append(env, "ORCON_FLOOR_POSTGRES_DSN="+dsn)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-run", "^TestFloorSmoke$", "-v", "./internal/adapter/storetest/")
	cmd.Dir = dir
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	t.Logf("floor %s in %s:\n%s", version, dir, out.String())
	if err != nil {
		t.Fatalf("the smoke subset of release %s fails against the schema migrated by this tree: %v", version, err)
	}
	if !strings.Contains(out.String(), "--- PASS: TestFloorSmoke/sqlite") {
		t.Fatalf("the smoke subset of release %s did not run on SQLite", version)
	}
	if postgresEnabled() && wantPostgres() && !strings.Contains(out.String(), "--- PASS: TestFloorSmoke/postgres") {
		t.Fatalf("the smoke subset of release %s did not run on PostgreSQL", version)
	}
}

// TestCompatFloorMachinery runs the floor procedure with this very tree as the
// "older release": it proves that preparing a database with one code base and
// running the smoke subset of another against it works, so the script does not
// pass vacuously when no migration declares a floor yet.
func TestCompatFloorMachinery(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	floorRun(t, root, binaryVersionDefault)
}

// TestCompatFloor runs the procedure for each release checkout the script
// prepared: ORCON_COMPAT_FLOORS holds "<version>=<directory>" pairs separated by
// semicolons, one per distinct compatibility floor declared by a migration
// (make test-compat-floor).
func TestCompatFloor(t *testing.T) {
	list := os.Getenv("ORCON_COMPAT_FLOORS")
	if list == "" {
		t.Skip("no migration declares a compatibility floor other than self (make test-compat-floor)")
	}
	for _, pair := range strings.Split(list, ";") {
		version, dir, ok := strings.Cut(pair, "=")
		if !ok || version == "" || dir == "" {
			t.Fatalf("ORCON_COMPAT_FLOORS entry %q is not <version>=<directory>", pair)
		}
		t.Run(version, func(t *testing.T) { floorRun(t, dir, version) })
	}
}
