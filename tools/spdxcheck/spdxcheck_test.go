// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	const (
		goHeader = "// SPDX-FileCopyrightText: The Orcon Authors\n// SPDX-License-Identifier: Apache-2.0\n"
		mdHeader = "<!-- SPDX-FileCopyrightText: The Orcon Authors -->\n<!-- SPDX-License-Identifier: Apache-2.0 -->\n"
	)
	tests := []struct {
		name         string
		path         string
		content      string
		wantFindings int
		wantFixed    string // empty when no fix is expected
		wantMsg      string
	}{
		{name: "go ok", path: "a.go", content: goHeader + "\npackage a\n"},
		{name: "go missing", path: "a.go", content: "package a\n", wantFindings: 2, wantFixed: goHeader + "\npackage a\n"},
		{
			name: "go doc comment stays separate", path: "a.go", content: "// Package a does things.\npackage a\n",
			wantFindings: 2, wantFixed: goHeader + "\n// Package a does things.\npackage a\n",
		},
		{
			name: "different holder is a finding", path: "a.go",
			content:      "// SPDX-FileCopyrightText: 2026 Someone Else\n// SPDX-License-Identifier: Apache-2.0\n\npackage a\n",
			wantFindings: 1, wantFixed: goHeader + "\npackage a\n", wantMsg: "copyright holder",
		},
		{
			name: "holder with year is a finding", path: "a.go",
			content:      "// SPDX-FileCopyrightText: 2026 The Orcon Authors\n// SPDX-License-Identifier: Apache-2.0\n\npackage a\n",
			wantFindings: 1, wantFixed: goHeader + "\npackage a\n",
		},
		{
			name: "wrong license is a finding", path: "a.go",
			content:      "// SPDX-FileCopyrightText: The Orcon Authors\n// SPDX-License-Identifier: MIT\n\npackage a\n",
			wantFindings: 1, wantFixed: goHeader + "\npackage a\n", wantMsg: "license",
		},
		{
			name: "only license present", path: "a.go",
			content:      "// SPDX-License-Identifier: Apache-2.0\n\npackage a\n",
			wantFindings: 1, wantFixed: "// SPDX-FileCopyrightText: The Orcon Authors\n// SPDX-License-Identifier: Apache-2.0\n\npackage a\n",
		},
		{name: "shell keeps shebang first", path: "x.sh", content: "#!/bin/sh\necho hi\n", wantFindings: 2, wantFixed: "#!/bin/sh\n# SPDX-FileCopyrightText: The Orcon Authors\n# SPDX-License-Identifier: Apache-2.0\n\necho hi\n"},
		{name: "extensionless script with shebang", path: "hooks/commit-msg", content: "#!/bin/sh\nexit 0\n", wantFindings: 2, wantFixed: "#!/bin/sh\n# SPDX-FileCopyrightText: The Orcon Authors\n# SPDX-License-Identifier: Apache-2.0\n\nexit 0\n"},
		{name: "makefile", path: "Makefile", content: "all:\n", wantFindings: 2, wantFixed: "# SPDX-FileCopyrightText: The Orcon Authors\n# SPDX-License-Identifier: Apache-2.0\n\nall:\n"},
		{name: "go.mod uses slashes", path: "go.mod", content: "module x\n", wantFindings: 2, wantFixed: goHeader + "\nmodule x\n"},
		{name: "markdown", path: "d.md", content: "# Title\n", wantFindings: 2, wantFixed: mdHeader + "# Title\n"},
		{name: "markdown ok", path: "d.md", content: mdHeader + "# Title\n"},
		{
			name: "markdown front matter first", path: "d.md", content: "---\nname: x\n---\n# Title\n",
			wantFindings: 2, wantFixed: "---\nname: x\n---\n" + mdHeader + "# Title\n",
		},
		{
			name: "markdown header after front matter ok", path: "d.md",
			content: "---\nname: x\n---\n" + mdHeader + "# Title\n",
		},
		{name: "yaml ok", path: "c.yml", content: "# SPDX-FileCopyrightText: The Orcon Authors\n# SPDX-License-Identifier: Apache-2.0\nkey: 1\n"},
		{name: "unsupported type", path: "data.json", content: "{}\n", wantFindings: 1, wantMsg: "unsupported file type"},
		{name: "binary is skipped", path: "logo.png", content: "\x89PNG\x00\x01"},
		{name: "empty go file", path: "a.go", content: "", wantFindings: 2, wantFixed: goHeader},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Check(tt.path, []byte(tt.content))
			if len(res.Findings) != tt.wantFindings {
				t.Fatalf("findings = %v, want %d", res.Findings, tt.wantFindings)
			}
			if tt.wantMsg != "" && !strings.Contains(res.Findings[0].Message, tt.wantMsg) {
				t.Fatalf("message %q does not contain %q", res.Findings[0].Message, tt.wantMsg)
			}
			if tt.wantFixed == "" {
				if res.Fixed != nil && tt.wantFindings == 0 {
					t.Fatalf("unexpected fix for a clean file: %q", res.Fixed)
				}
				return
			}
			if string(res.Fixed) != tt.wantFixed {
				t.Fatalf("fixed content =\n%q\nwant\n%q", res.Fixed, tt.wantFixed)
			}
			// Fixing is idempotent: the fixed content has no findings.
			if again := Check(tt.path, res.Fixed); len(again.Findings) != 0 {
				t.Fatalf("fixed content still has findings: %v", again.Findings)
			}
		})
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.go")
	bad := filepath.Join(dir, "bad.go")
	skipped := filepath.Join(dir, "skip.json")
	exempt := filepath.Join(dir, "exempt")
	writeFile(t, good, "// SPDX-FileCopyrightText: The Orcon Authors\n// SPDX-License-Identifier: Apache-2.0\n\npackage a\n")
	writeFile(t, bad, "package a\n")
	writeFile(t, skipped, "{}\n")
	writeFile(t, exempt, "# comment\n"+filepath.ToSlash(skipped)+"  # JSON has no comment syntax\n")

	var out, errOut bytes.Buffer
	if code := run([]string{"-exempt", exempt, good, skipped}, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("clean run = %d, stdout %q stderr %q", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := run([]string{"-exempt", exempt, bad}, strings.NewReader(""), &out, &errOut); code != 1 {
		t.Fatalf("run with a bad file = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "bad.go: missing") {
		t.Fatalf("stdout = %q, want a missing-header finding", out.String())
	}
	if code := run([]string{"-fix", "-exempt", exempt}, strings.NewReader(bad+"\n"), &out, &errOut); code != 0 {
		t.Fatalf("fix run = %d", code)
	}
	if code := run([]string{"-exempt", exempt, bad}, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("run after fix = %d, want 0", code)
	}
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
