// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// TestQueryVerbsEqualNM14 keeps policy/query_verbs.txt equal to the verb list
// NM-14 gives: changing one means changing the other in the same commit.
func TestQueryVerbsEqualNM14(t *testing.T) {
	data, err := os.ReadFile("../../.agents/skills/orcon-db/references/naming.md")
	if err != nil {
		t.Fatal(err)
	}
	var line string
	for _, l := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(l, "- **NM-14**") {
			line = l
		}
	}
	if line == "" {
		t.Fatal("NM-14 not found in naming.md")
	}
	start := strings.Index(line, "from:")
	list := line[start:]
	if i := strings.Index(list, "(the checkers"); i > 0 {
		list = list[:i]
	}
	var want []string
	for _, m := range regexp.MustCompile("`([A-Z][a-z]+)`").FindAllStringSubmatch(list, -1) {
		want = append(want, m[1])
	}
	p, fs := loadPolicy(realPolicy)
	if len(fs) != 0 {
		t.Fatalf("policy: %v", fs)
	}
	var got []string
	for v := range p.queryVerbs {
		got = append(got, v)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(want) != 18 || !slices.Equal(got, want) {
		t.Fatalf("query_verbs.txt = %v, NM-14 lists %v (want 18 equal verbs)", got, want)
	}
}

var digestRE = regexp.MustCompile(`postgres:(\d+)@(sha256:[0-9a-f]{64})`)

func digests(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, m := range digestRE.FindAllStringSubmatch(string(data), -1) {
		if prev, ok := out[m[1]]; ok && prev != m[2] {
			t.Fatalf("%s pins postgres:%s twice with different digests", path, m[1])
		}
		out[m[1]] = m[2]
	}
	return out
}

// TestPostgresImageDigestsMatch keeps the Makefile and the CI matrix on the
// same PostgreSQL image digests, and CI free of the developer-only server override.
func TestPostgresImageDigestsMatch(t *testing.T) {
	mk := digests(t, "../../Makefile")
	ci := digests(t, "../../.github/workflows/ci.yml")
	if len(mk) != 2 || mk["14"] == "" || mk["17"] == "" {
		t.Fatalf("Makefile must pin postgres 14 and 17 by digest, got %v", mk)
	}
	for v, d := range mk {
		if ci[v] != d {
			t.Errorf("postgres:%s digest differs: Makefile %s, ci.yml %s", v, d, ci[v])
		}
	}
	data, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "ORCON_TEST_POSTGRES_DSN") {
		t.Error("CI must never set ORCON_TEST_POSTGRES_DSN")
	}
}
