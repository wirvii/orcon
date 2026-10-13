// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// repoFixture is a temporary git repository with a base commit on main.
type repoFixture struct {
	t    *testing.T
	root string
}

func newRepo(t *testing.T, base map[string]string) *repoFixture {
	t.Helper()
	r := &repoFixture{t: t, root: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.write(".gitattributes", "*.lock pr-size-exclude\ngen/** linguist-generated\n")
	for p, c := range base {
		r.write(p, c)
	}
	r.commit("base")
	r.git("checkout", "-q", "-b", "topic")
	return r
}

func (r *repoFixture) git(args ...string) {
	r.t.Helper()
	cmd := exec.CommandContext(r.t.Context(), "git", append([]string{"-C", r.root,
		"-c", "user.name=T", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (r *repoFixture) write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		r.t.Fatal(err)
	}
}

func (r *repoFixture) commit(msg string) {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "-m", msg)
}

func lines(n int) string {
	return strings.Repeat("x\n", n)
}

func measure(t *testing.T, r *repoFixture) SizeResult {
	t.Helper()
	res, err := MeasureSize(t.Context(), r.root, "main", "topic")
	if err != nil {
		t.Fatalf("MeasureSize: %v", err)
	}
	return res
}

func TestMeasureSize(t *testing.T) {
	t.Parallel()
	t.Run("new file counts its lines", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t, nil)
		r.write("code.go", lines(12))
		r.commit("add")
		if got := measure(t, r); got.Counted != 12 || got.Excluded != 0 {
			t.Fatalf("got %+v, want 12 counted", got)
		}
	})
	t.Run("added plus deleted", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t, map[string]string{"a.txt": "one\ntwo\nthree\n"})
		r.write("a.txt", "one\nTWO\nthree\nfour\n")
		r.commit("edit")
		// One line replaced (1 added + 1 deleted) and one added.
		if got := measure(t, r); got.Counted != 3 {
			t.Fatalf("got %+v, want 3 counted", got)
		}
	})
	t.Run("pr-size-exclude attribute excludes", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t, nil)
		r.write("deps.lock", lines(500))
		r.write("main.go", lines(5))
		r.commit("add")
		if got := measure(t, r); got.Counted != 5 || got.Excluded != 500 {
			t.Fatalf("got %+v, want 5 counted and 500 excluded", got)
		}
	})
	t.Run("linguist-generated attribute excludes", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t, nil)
		r.write("gen/out.go", lines(300))
		r.write("main.go", lines(7))
		r.commit("add")
		if got := measure(t, r); got.Counted != 7 || got.Excluded != 300 {
			t.Fatalf("got %+v, want 7 counted and 300 excluded", got)
		}
	})
	t.Run("binary file counts zero", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t, nil)
		r.write("image.bin", "\x00\x01\x02\x03binary\x00")
		r.write("main.go", lines(2))
		r.commit("add")
		got := measure(t, r)
		if got.Counted != 2 {
			t.Fatalf("got %+v, want 2 counted", got)
		}
		if !slices.Contains(got.ChangedPaths(), "image.bin") {
			t.Fatalf("the binary file is a changed path: %v", got.ChangedPaths())
		}
	})
	t.Run("rename without edits counts zero and reports the new path", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t, map[string]string{"old.go": lines(50)})
		r.git("mv", "old.go", "new.go")
		r.commit("rename")
		got := measure(t, r)
		if got.Counted != 0 {
			t.Fatalf("got %+v, want 0 counted", got)
		}
		if !slices.Equal(got.ChangedPaths(), []string{"new.go"}) {
			t.Fatalf("paths = %v, want [new.go]", got.ChangedPaths())
		}
	})
	t.Run("rename with an edit counts the edit", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t, map[string]string{"old.go": lines(50)})
		r.git("mv", "old.go", "new.go")
		r.write("new.go", lines(50)+"extra\n")
		r.commit("rename and edit")
		if got := measure(t, r); got.Counted != 1 {
			t.Fatalf("got %+v, want 1 counted", got)
		}
	})
	t.Run("empty diff", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t, nil)
		if got := measure(t, r); got.Counted != 0 || len(got.Files) != 0 {
			t.Fatalf("got %+v, want empty", got)
		}
	})
}

func TestSizeLimitBoundary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		n        int
		wantWarn bool
	}{
		{"400 does not warn", 400, false},
		{"401 warns", 401, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t, nil)
			r.write("big.go", lines(tt.n))
			r.commit("add")
			var stdout, stderr bytes.Buffer
			code := run(t.Context(), []string{"size", "--repo", r.root, "--base", "main", "--head", "topic", "--summary", ""}, &stdout, &stderr)
			if code != exitOK {
				t.Fatalf("exit %d, stderr %q", code, stderr.String())
			}
			if got := strings.Contains(stdout.String(), "::warning"); got != tt.wantWarn {
				t.Fatalf("warning = %v, want %v\n%s", got, tt.wantWarn, stdout.String())
			}
		})
	}
}

func TestHasSizeJustification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"valid", "## Size justification\n\nThe change cannot be split because the schema and the code move together.\n\n## Other\n", true},
		{"no heading", "## What and why\n\nSomething long enough to count as text here.\n", false},
		{"empty section", "## Size justification\n\n## Breaking changes\nNone\n", false},
		{"only a one-line comment", "## Size justification\n<!-- Explain why the change cannot be split, at least one sentence. -->\n\n## Next\n", false},
		{"only a multi-line comment", "## Size justification\n<!--\nExplain why the change cannot be split in at least one sentence.\n-->\n## Next\n", false},
		{"too short", "## Size justification\n\ntoo short\n", false},
		{"comment then text", "## Size justification\n<!-- hint -->\nThe generated catalogs and the code must land together.\n", true},
		{"text after the next heading does not count", "## Size justification\n\n## Next\nA long enough sentence that sits under another heading.\n", false},
		{"last section", "intro\n## Size justification\nA sentence long enough to explain the size of this change.", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HasSizeJustification(tt.body); got != tt.want {
				t.Fatalf("HasSizeJustification = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckSpecLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"valid spec", "intro\n\nSpec: SPEC-013\n", false},
		{"valid long number", "Spec: SPEC-0123\n", false},
		{"n/a", "Spec: n/a\n", false},
		{"windows line ending", "Spec: SPEC-013\r\nnext\n", false},
		{"absent", "## What and why\nnothing\n", true},
		{"two digits", "Spec: SPEC-13\n", true},
		{"lowercase", "Spec: spec-013\n", true},
		{"trailing text", "Spec: SPEC-013 and more\n", true},
		{"indented", "  Spec: SPEC-013\n", true},
		{"empty", "Spec:\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := CheckSpecLine(tt.body); (err != nil) != tt.wantErr {
				t.Fatalf("CheckSpecLine err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckBranchName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		branch  string
		wantErr bool
	}{
		{"feature", "feat/spec-013-tooling", false},
		{"fix", "fix/join-deadlock", false},
		{"revert", "revert/bad-change", false},
		{"release bot branch", "release-please--branches--main", false},
		{"digits", "chore/go-1-26", false},
		{"unknown type", "feature/x", true},
		{"no slash", "my-branch", true},
		{"uppercase", "feat/Spec-013", true},
		{"underscore", "feat/my_branch", true},
		{"double hyphen", "feat/a--b", true},
		{"trailing hyphen", "feat/a-", true},
		{"nested slash", "feat/a/b", true},
		{"too long", "feat/" + strings.Repeat("a", 56), true},
		{"exactly 60", "feat/" + strings.Repeat("a", 55), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := CheckBranchName(tt.branch); (err != nil) != tt.wantErr {
				t.Fatalf("CheckBranchName(%q) err = %v, wantErr %v", tt.branch, err, tt.wantErr)
			}
		})
	}
}

func TestApplicableSkills(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		changed []string
		want    []string
	}{
		{"nothing", nil, nil},
		{"docs only", []string{"docs/adr/0001.md", "README.md"}, nil},
		{"go code", []string{"internal/flow/flow.go"}, []string{"orcon-go"}},
		{"go module file", []string{"tools/go.mod"}, []string{"orcon-go"}},
		{"web app", []string{"apps/web/src/main.tsx"}, []string{"orcon-ui", "orcon-web"}},
		{"js package", []string{"packages/js/api-contract/src/index.ts"}, []string{"orcon-web"}},
		{"sql", []string{"internal/adapter/sqlite/migrations/0001.sql"}, []string{"orcon-db"}},
		{"api", []string{"packages/openapi/v1/openapi.yaml"}, []string{"orcon-api"}},
		{"httpapi go", []string{"internal/adapter/httpapi/server.go"}, []string{"orcon-go", "orcon-api"}},
		{"connector", []string{"internal/adapter/connectors/http/http.go"}, []string{"orcon-go", "orcon-connector"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ApplicableSkills(tt.changed)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("ApplicableSkills(%v) = %v, want %v", tt.changed, got, tt.want)
			}
		})
	}
}

func TestMissingChecklists(t *testing.T) {
	t.Parallel()
	body := "- [x] orcon-go checklist pasted below\n- [ ] orcon-db checklist pasted below\n"
	got := MissingChecklists(body, []string{"orcon-go", "orcon-db"})
	if !slices.Equal(got, []string{"orcon-db"}) {
		t.Fatalf("MissingChecklists = %v, want [orcon-db]", got)
	}
}

// TestSkillMapMatchesRulebook keeps SkillMap equal to the table PR-6 of the
// rulebook (orcon-contributing pull-requests.md).
func TestSkillMapMatchesRulebook(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.FromSlash("../../.agents/skills/orcon-contributing/references/pull-requests.md"))
	if err != nil {
		t.Fatalf("read the rulebook: %v", err)
	}
	rowPattern := regexp.MustCompile("(?m)^\\| `(orcon-[a-z]+)` \\| (.+) \\|$")
	globPattern := regexp.MustCompile("`([^`]+)`")
	documented := map[string][]string{}
	for _, m := range rowPattern.FindAllStringSubmatch(string(raw), -1) {
		for _, g := range globPattern.FindAllStringSubmatch(m[2], -1) {
			documented[m[1]] = append(documented[m[1]], g[1])
		}
	}
	if len(documented) != len(SkillMap) {
		t.Fatalf("the rulebook lists %d skills, SkillMap has %d", len(documented), len(SkillMap))
	}
	for skill, want := range documented {
		got := SkillMap[skill]
		if !slices.Equal(got, want) {
			t.Errorf("%s: SkillMap = %v, rulebook = %v", skill, got, want)
		}
	}
	for _, skill := range SkillOrder {
		if _, ok := SkillMap[skill]; !ok {
			t.Errorf("SkillOrder names %s, which SkillMap lacks", skill)
		}
	}
}

func TestRunSkillsList(t *testing.T) {
	t.Parallel()
	r := newRepo(t, nil)
	r.write("internal/flow/a.go", lines(1))
	r.write("docs/x.md", lines(1))
	r.commit("add")
	var stdout, stderr bytes.Buffer
	code := run(t.Context(), []string{"skills", "--list", "--repo", r.root, "--base", "main", "--head", "topic"}, &stdout, &stderr)
	if code != exitOK || stdout.String() != "orcon-go\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestRunAllReportsWarnings(t *testing.T) {
	t.Parallel()
	r := newRepo(t, nil)
	r.write("internal/flow/a.go", lines(2))
	r.commit("add")
	bodyFile := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(bodyFile, []byte("## What and why\nx\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	summary := filepath.Join(t.TempDir(), "summary.md")
	var stdout, stderr bytes.Buffer
	args := []string{"all", "--repo", r.root, "--base", "main", "--head", "topic", "--body-file", bodyFile, "--branch", "My_Branch", "--summary", summary}
	if code := run(t.Context(), args, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"Spec line", "Branch name", "Skill checklists: tick"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "::warning"); n != 3 {
		t.Errorf("got %d warnings, want 3 (spec, branch, skills):\n%s", n, out)
	}
	written, err := os.ReadFile(summary) //nolint:gosec // path from t.TempDir
	if err != nil || !strings.Contains(string(written), "| Size |") {
		t.Fatalf("summary not written: %v %q", err, written)
	}
}

func TestRunErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
	}{
		{"no command", nil},
		{"unknown command", []string{"nope"}},
		{"unknown flag", []string{"size", "--nope"}},
		{"not a repository", []string{"size", "--repo", t.TempDir(), "--base", "main", "--head", "topic"}},
		{"missing body file", []string{"spec", "--body-file", filepath.Join(t.TempDir(), "missing.md")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if code := run(t.Context(), tt.args, &stdout, &stderr); code != exitUsage {
				t.Fatalf("exit %d, want %d", code, exitUsage)
			}
		})
	}
}
