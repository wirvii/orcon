// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// wantBadCases is the number of planted violations of SPEC-009 4.4.6.
const wantBadCases = 26

func fixtureConfig(dir string) config {
	return config{
		SQLiteDir:   filepath.Join(dir, "sqlite"),
		PostgresDir: filepath.Join(dir, "postgres"),
		PolicyDir:   realPolicy,
	}
}

func TestGoodFixturesPass(t *testing.T) {
	dirs, err := filepath.Glob("testdata/good/*")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no good fixtures found (%v)", err)
	}
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			findings, _, code := runCheck(t, "all", fixtureConfig(dir))
			if code != 0 || len(findings) != 0 {
				t.Fatalf("exit %d, findings:\n%s", code, strings.Join(findings, "\n"))
			}
		})
	}
}

func TestBadFixturesAreDetected(t *testing.T) {
	dirs, err := filepath.Glob("testdata/bad/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != wantBadCases {
		t.Fatalf("found %d planted cases, SPEC-009 4.4.6 requires %d", len(dirs), wantBadCases)
	}
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, "expect.txt"))
			if err != nil {
				t.Fatal(err)
			}
			want := strings.Fields(string(raw))
			sort.Strings(want)
			findings, _, code := runCheck(t, "all", fixtureConfig(dir))
			if code != 1 {
				t.Fatalf("the planted violation was not detected (exit %d)", code)
			}
			if got := rulesOf(findings); !reflect.DeepEqual(got, want) {
				t.Fatalf("rules = %v, want exactly %v\n%s", got, want, strings.Join(findings, "\n"))
			}
		})
	}
}

// Every check of 4.4.1 must be exercised by at least one planted case or table
// case; the planted cases alone cover these checks.
func TestEachPlantedCaseNamesItsCheck(t *testing.T) {
	covered := map[string]bool{}
	dirs, _ := filepath.Glob("testdata/bad/*")
	for _, dir := range dirs {
		for _, c := range checks {
			findings, _, _ := runCheck(t, c.name, fixtureConfig(dir))
			if len(findings) > 0 {
				covered[c.name] = true
			}
		}
	}
	// migration-headers fires on the missing-floor case; policy has no planted
	// case on purpose (malformed lists are unit tests, SPEC-009 4.4.6).
	for _, c := range checks {
		if c.name == "policy" {
			continue
		}
		if !covered[c.name] {
			t.Errorf("no planted case triggers %s", c.name)
		}
	}
}
