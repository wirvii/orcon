// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGit answers git commands from a table keyed by the joined arguments.
type fakeGit struct {
	answers map[string]string
	errs    map[string]error
	calls   []string
}

func (f *fakeGit) Run(_ string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if err, ok := f.errs[key]; ok {
		return nil, err
	}
	if out, ok := f.answers[key]; ok {
		return []byte(out), nil
	}
	return nil, fmt.Errorf("fakeGit: unexpected command %q", key)
}

func baseSum(lines ...string) string { return "# header\n" + strings.Join(lines, "\n") + "\n" }

func sumLineFor(name, content string) string { return name + "  " + fileHash([]byte(content)) }

func TestImmutableComparesWithTheBase(t *testing.T) {
	files := pair(1, "a", additive, "SELECT 1;\n")
	cur := withSums(files)
	curLine := sumLineFor("0001_a.sql", files["sqlite/migrations/0001_a.sql"])
	pgLine := sumLineFor("0001_a.sql", files["postgres/migrations/0001_a.sql"])
	other := sumLineFor("0001_a.sql", "something else")
	gone := sumLineFor("0000_gone.sql", "x")

	tests := []struct {
		name     string
		sqlite   string // migrations.sum at the base, "" = file absent
		postgres string
		rules    []string
		contains string
	}{
		{"same lines", baseSum(curLine), baseSum(pgLine), nil, ""},
		{"base without lines", baseSum(), baseSum(), nil, ""},
		{"base line changed", baseSum(other), baseSum(pgLine), []string{"MG-5"}, "differs from base"},
		{"base line removed", baseSum(curLine, gone), baseSum(pgLine), []string{"MG-5"}, "exists in base"},
		{"base file absent", "", "", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tree(t, cur, nil)
			g := &fakeGit{answers: map[string]string{"rev-parse --verify --quiet BASE^{commit}": "abc123\n"}}
			cfg.Git = &perDirGit{base: g, sums: map[string]string{cfg.SQLiteDir + "/migrations": tt.sqlite, cfg.PostgresDir + "/migrations": tt.postgres}}
			cfg.Base = "BASE"
			findings, notes, code := runCheck(t, "migration-immutable", cfg)
			if got := rulesOf(findings); len(got) != len(tt.rules) || (len(got) > 0 && got[0] != tt.rules[0]) {
				t.Fatalf("rules = %v, want %v\n%s", got, tt.rules, strings.Join(findings, "\n"))
			}
			if (code == 1) != (len(tt.rules) > 0) {
				t.Fatalf("exit code %d", code)
			}
			if tt.contains != "" && !contains(findings, tt.contains) {
				t.Fatalf("findings lack %q: %v", tt.contains, findings)
			}
			if tt.sqlite == "" && !contains(notes, "does not exist at base") {
				t.Fatalf("an absent base file was not declared: %v", notes)
			}
		})
	}
}

// perDirGit routes the git commands of one migrations directory to its own
// answers: dir is the -C argument the tool passes.
type perDirGit struct {
	base *fakeGit
	sums map[string]string
}

func (p *perDirGit) Run(dir string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(key, "rev-parse"):
		return p.base.Run(dir, args...)
	case strings.HasPrefix(key, "ls-tree --name-only abc123 -- ./migrations.sum"):
		if p.sums[dir] == "" {
			return nil, nil
		}
		return []byte("migrations.sum\n"), nil
	case strings.HasPrefix(key, "show abc123:./migrations.sum"):
		return []byte(p.sums[dir]), nil
	}
	return nil, fmt.Errorf("perDirGit: unexpected command %q in %s", key, dir)
}

func TestImmutableBaseThatCannotBeResolvedIsAFinding(t *testing.T) {
	cfg := tree(t, withSums(pair(1, "a", additive, "")), nil)
	cfg.Git = &fakeGit{errs: map[string]error{"rev-parse --verify --quiet nope^{commit}": errors.New("exit status 1")}}
	cfg.Base = "nope"
	findings, _, code := runCheck(t, "migration-immutable", cfg)
	if code != 1 || !contains(findings, `base revision "nope" cannot be resolved`) {
		t.Fatalf("a base that was asked for and cannot be resolved must fail: %v", findings)
	}
}

func TestImmutableWithoutOriginMainIsDeclared(t *testing.T) {
	cfg := tree(t, withSums(pair(1, "a", additive, "")), nil)
	cfg.Git = &fakeGit{errs: map[string]error{"merge-base HEAD origin/main": errors.New("fatal: Not a valid object name origin/main")}}
	findings, notes, code := runCheck(t, "migration-immutable", cfg)
	if code != 0 || len(findings) != 0 {
		t.Fatalf("exit %d: %v", code, findings)
	}
	if !contains(notes, "no base revision available") {
		t.Fatalf("the missing base was not declared: %v", notes)
	}
}

func TestContractNeedsAnEarlierRelease(t *testing.T) {
	k := func(kind string) string { return "-- orcon:kind " + kind + "\n-- orcon:compatibility-floor self\n" }
	files := withSums(merge(
		pair(1, "a", k("expand"), "ALTER TABLE a ADD COLUMN b TEXT;\n"),
		pair(2, "b", k("contract")+"-- orcon:contracts 0001\n", "ALTER TABLE a DROP COLUMN c;\n")))
	tagCmd := "tag --list v* --sort=-v:refname"
	lsCmd := "ls-tree --name-only v1.0.0 -- ./migrations/"

	t.Run("no release tags is declared not applicable", func(t *testing.T) {
		cfg := tree(t, files, nil)
		cfg.Git = &fakeGit{answers: map[string]string{tagCmd: ""}}
		findings, notes, code := runCheck(t, "migration-expand-contract", cfg)
		if code != 0 || len(findings) != 0 {
			t.Fatalf("exit %d: %v", code, findings)
		}
		if !contains(notes, "no release tags") {
			t.Fatalf("notes: %v", notes)
		}
	})
	t.Run("expand shipped in a release", func(t *testing.T) {
		cfg := tree(t, files, nil)
		cfg.Git = &fakeGit{answers: map[string]string{tagCmd: "v1.0.0\n", lsCmd: "migrations/0001_a.sql\n"}}
		if findings, _, code := runCheck(t, "migration-expand-contract", cfg); code != 0 {
			t.Fatalf("findings: %v", findings)
		}
	})
	t.Run("expand not shipped", func(t *testing.T) {
		cfg := tree(t, files, nil)
		cfg.Git = &fakeGit{answers: map[string]string{tagCmd: "v1.0.0\n", lsCmd: "migrations/0000_other.sql\n"}}
		findings, _, code := runCheck(t, "migration-expand-contract", cfg)
		if code != 1 || !contains(findings, "was not shipped in release v1.0.0") {
			t.Fatalf("findings: %v", findings)
		}
	})
	t.Run("git unavailable is declared", func(t *testing.T) {
		cfg := tree(t, files, nil)
		_, notes, code := runCheck(t, "migration-expand-contract", cfg)
		if code != 0 || !contains(notes, "not applicable") {
			t.Fatalf("exit %d notes %v", code, notes)
		}
	})
}

func TestSumAppendsNewLinesOnly(t *testing.T) {
	files := withSums(pair(1, "a", additive, "SELECT 1;\n"))
	cfg := tree(t, files, nil)
	// A second migration appears after the sums were written.
	for _, d := range []string{"sqlite", "postgres"} {
		path := filepath.Join(cfg.SQLiteDir, "..", d, "migrations", "0002_b.sql")
		if err := os.WriteFile(path, []byte(sqlHeader+additive+"SELECT 2;\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sumPath := filepath.Join(cfg.SQLiteDir, "migrations", "migrations.sum")
	before, _ := os.ReadFile(sumPath)
	findings, _, code := runCheck(t, "sum", cfg)
	if code != 0 {
		t.Fatalf("sum exit %d: %v", code, findings)
	}
	after, _ := os.ReadFile(sumPath)
	if !strings.HasPrefix(string(after), string(before)) {
		t.Fatal("db-sum rewrote existing lines")
	}
	if !strings.Contains(string(after), "0002_b.sql  ") {
		t.Fatalf("the new migration was not appended:\n%s", after)
	}
	if again, _, _ := runCheck(t, "sum", cfg); len(again) != 0 {
		t.Fatalf("second run: %v", again)
	}
	again, _ := os.ReadFile(sumPath)
	if string(again) != string(after) {
		t.Fatal("db-sum is not idempotent")
	}
	if f, _, code := runCheck(t, "migration-immutable", cfg); code != 0 {
		t.Fatalf("after db-sum the tree must pass: %v", f)
	}
}

func TestSumRefusesAMalformedFile(t *testing.T) {
	files := withSums(pair(1, "a", additive, ""))
	files["sqlite/migrations/migrations.sum"] = "garbage line\n"
	cfg := tree(t, files, nil)
	var out, errOut strings.Builder
	if code := execute("sum", cfg, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "malformed") {
		t.Fatalf("code %d stderr %q", code, errOut.String())
	}
}

// TestWithRealGit runs the immutability check against a real repository: the
// base revision is a commit, the working tree edits a recorded line.
func TestWithRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed; the fake-runner tests cover the logic")
	}
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.test", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	files := withSums(pair(1, "a", additive, "SELECT 1;\n"))
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	base := git("rev-parse", "HEAD")
	git("tag", "v1.0.0")

	tags, err := releaseTags(execGit{}, root)
	if err != nil || len(tags) != 1 || tags[0] != "v1.0.0" {
		t.Fatalf("releaseTags = %v, %v", tags, err)
	}
	names, err := listDir(execGit{}, filepath.Join(root, "sqlite"), "v1.0.0", "migrations")
	if err != nil || !containsWord(names, "0001_a.sql") || !containsWord(names, "migrations.sum") {
		t.Fatalf("listDir = %v, %v", names, err)
	}

	cfg := tree(t, nil, nil)
	cfg.SQLiteDir = filepath.Join(root, "sqlite")
	cfg.PostgresDir = filepath.Join(root, "postgres")
	cfg.Git = execGit{}
	cfg.Base = base

	if findings, notes, code := runCheck(t, "migration-immutable", cfg); code != 0 {
		t.Fatalf("unchanged tree: %v %v", findings, notes)
	}

	// Rewrite the recorded line and make the file match it: only the base reveals it.
	edited := files["sqlite/migrations/0001_a.sql"] + "-- edited\n"
	if err := os.WriteFile(filepath.Join(root, "sqlite/migrations/0001_a.sql"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := "# header\n0001_a.sql  " + fileHash([]byte(edited)) + "\n"
	if err := os.WriteFile(filepath.Join(root, "sqlite/migrations/migrations.sum"), []byte(sum), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, _, code := runCheck(t, "migration-immutable", cfg)
	if code != 1 || !contains(findings, "differs from base") {
		t.Fatalf("an edit that rewrote its own checksum line was not caught: %v", findings)
	}

	// An unknown base is an error, not a pass.
	cfg.Base = "does-not-exist"
	if findings, _, code := runCheck(t, "migration-immutable", cfg); code != 1 || !contains(findings, "cannot be resolved") {
		t.Fatalf("unknown base: %v", findings)
	}
}

func TestBaseNoneComparesOnlyWithTheSums(t *testing.T) {
	cfg := tree(t, withSums(pair(1, "a", additive, "")), nil)
	cfg.Git = &fakeGit{}
	cfg.Base = "none"
	findings, notes, code := runCheck(t, "migration-immutable", cfg)
	if code != 0 || len(findings) != 0 || !contains(notes, "no base revision available") {
		t.Fatalf("exit %d findings %v notes %v", code, findings, notes)
	}
}
