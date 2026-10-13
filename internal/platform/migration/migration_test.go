// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package migration_test

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wirvii/orcon/internal/platform/migration"
)

func file(kind, floor string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte("-- SPDX-FileCopyrightText: The Orcon Authors\r\n-- SPDX-License-Identifier: Apache-2.0\r\n-- orcon:kind " + kind + "\r\n-- orcon:compatibility-floor " + floor + "\r\n\r\nSELECT 1;\r\n")}
}

func TestLoadParsesHeadersAndNormalizesLineEndings(t *testing.T) {
	fsys := fstest.MapFS{
		"m/0001_first.sql":       file("additive", "self"),
		"m/0002_second_step.sql": file("expand", "1.2.0"),
		"m/migrations.sum":       {Data: []byte("ignored")},
		"m/testdata/0001_seed.sql": {
			Data: []byte("ignored"),
		},
	}
	got, err := migration.Load(fsys, "m")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("loaded %d migrations, want 2", len(got))
	}
	if got[0].Floor != migration.FloorSelf || got[1].Floor != "1.2.0" || got[1].Kind != migration.KindExpand {
		t.Fatalf("headers not read: %+v", got)
	}
	if strings.Contains(got[0].SQL, "\r") {
		t.Fatal("SQL kept CRLF")
	}
	lf := strings.ReplaceAll(string(file("additive", "self").Data), "\r\n", "\n")
	if got[0].Checksum != migration.Checksum([]byte(lf)) {
		t.Fatal("checksum must not depend on line endings")
	}
	if got[0].RecordedFloor("0.3.0") != "0.3.0" || got[1].RecordedFloor("0.3.0") != "1.2.0" {
		t.Fatal("self must record the binary version, a declared floor itself")
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	cases := map[string]fstest.MapFS{
		"gap":          {"m/0001_a.sql": file("additive", "self"), "m/0003_c.sql": file("additive", "self")},
		"bad name":     {"m/1_a.sql": file("additive", "self")},
		"bad kind":     {"m/0001_a.sql": file("sideways", "self")},
		"bad floor":    {"m/0001_a.sql": file("additive", "soon")},
		"unknown key":  {"m/0001_a.sql": {Data: []byte("-- orcon:kind additive\n-- orcon:compatibility-floor self\n-- orcon:bogus x\n")}},
		"missing dir":  {},
		"missing kind": {"m/0001_a.sql": {Data: []byte("SELECT 1;\n")}},
	}
	for name, fsys := range cases {
		if _, err := migration.Load(fsys, "m"); err == nil {
			t.Errorf("%s: Load succeeded", name)
		}
	}
}

func TestVersionOrdering(t *testing.T) {
	order := []string{"0.0.0-dev", "0.1.0-alpha.1", "0.1.0-alpha.2", "0.1.0-beta", "0.1.0", "v0.1.1", "0.2.0+build5", "1.0.0"}
	for i, left := range order {
		for j, right := range order {
			a, err := migration.ParseVersion(left)
			if err != nil {
				t.Fatal(err)
			}
			b, err := migration.ParseVersion(right)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			if got := a.Compare(b); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", left, right, got, want)
			}
		}
	}
	for _, bad := range []string{"", "1", "1.2", "01.2.3", "1.2.3-", "dev"} {
		if _, err := migration.ParseVersion(bad); err == nil {
			t.Errorf("ParseVersion(%q) succeeded", bad)
		}
	}
}

func embedded() []migration.Migration {
	return []migration.Migration{
		{Version: 1, Name: "0001_a.sql", Checksum: "aa"},
		{Version: 2, Name: "0002_b.sql", Checksum: "bb"},
		{Version: 3, Name: "0003_c.sql", Checksum: "cc"},
	}
}

func TestCheck(t *testing.T) {
	applied := func(floors ...string) []migration.Applied {
		var out []migration.Applied
		sums := map[int]string{1: "aa", 2: "bb", 3: "cc", 4: "dd"}
		for i, f := range floors {
			out = append(out, migration.Applied{Version: i + 1, Checksum: sums[i+1], BinaryVersion: "1.0.0", Floor: f})
		}
		return out
	}
	t.Run("empty database applies everything", func(t *testing.T) {
		p, err := migration.Check("1.0.0", nil, embedded())
		if err != nil || len(p.Pending) != 3 || p.EffectiveFloor != "" {
			t.Fatalf("plan %+v, err %v", p, err)
		}
	})
	t.Run("partial history applies the rest", func(t *testing.T) {
		p, err := migration.Check("1.0.0", applied("1.0.0", "1.0.0"), embedded())
		if err != nil || len(p.Pending) != 1 || p.Pending[0].Version != 3 || p.SchemaVersion != 2 {
			t.Fatalf("plan %+v, err %v", p, err)
		}
	})
	t.Run("binary below the effective floor refuses and names three versions", func(t *testing.T) {
		_, err := migration.Check("1.0.0", applied("0.9.0", "1.1.0", "0.9.0", "0.9.0")[:3], embedded())
		var fe *migration.FloorError
		if !errors.Is(err, migration.ErrBinaryTooOld) || !errors.As(err, &fe) {
			t.Fatalf("error %v", err)
		}
		if fe.BinaryVersion != "1.0.0" || fe.SchemaVersion != 3 || fe.Required != "1.1.0" {
			t.Fatalf("floor error %+v", fe)
		}
		for _, want := range []string{"1.0.0", "0003", "1.1.0"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("message %q lacks %q", err, want)
			}
		}
	})
	t.Run("binary older than the schema but at the floor runs", func(t *testing.T) {
		h := append(applied("0.9.0", "0.9.0", "0.9.0"), migration.Applied{Version: 4, Checksum: "dd", Floor: "1.0.0"})
		p, err := migration.Check("1.0.0", h, embedded())
		if err != nil || len(p.Pending) != 0 || p.SchemaVersion != 4 {
			t.Fatalf("plan %+v, err %v", p, err)
		}
	})
	t.Run("checksum mismatch refuses", func(t *testing.T) {
		h := applied("1.0.0", "1.0.0")
		h[1].Checksum = "tampered"
		_, err := migration.Check("1.0.0", h, embedded())
		if !errors.Is(err, migration.ErrChecksumMismatch) {
			t.Fatalf("error %v", err)
		}
	})
	t.Run("a hole in the history is invalid", func(t *testing.T) {
		h := []migration.Applied{{Version: 1, Checksum: "aa", Floor: "1.0.0"}, {Version: 3, Checksum: "cc", Floor: "1.0.0"}}
		if _, err := migration.Check("1.0.0", h, embedded()); !errors.Is(err, migration.ErrInvalidHistory) {
			t.Fatalf("error %v", err)
		}
	})
	t.Run("an unreadable floor is invalid", func(t *testing.T) {
		h := []migration.Applied{{Version: 1, Checksum: "aa", Floor: "self"}}
		if _, err := migration.Check("1.0.0", h, embedded()); !errors.Is(err, migration.ErrInvalidHistory) {
			t.Fatalf("error %v", err)
		}
	})
	t.Run("a bad binary version is an error", func(t *testing.T) {
		if _, err := migration.Check("dev", nil, embedded()); err == nil {
			t.Fatal("accepted")
		}
	})
}
