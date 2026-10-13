// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"path"
	"slices"
	"strings"
)

const (
	copyrightLine = "SPDX-FileCopyrightText: The Orcon Authors"
	licenseLine   = "SPDX-License-Identifier: Apache-2.0"

	copyrightTag = "SPDX-FileCopyrightText:"
	licenseTag   = "SPDX-License-Identifier:"

	// headerWindow is how many lines after the optional shebang or front
	// matter may hold the SPDX header.
	headerWindow = 12
)

// style is how a file type spells a comment.
type style int

const (
	styleNone  style = iota // no comment syntax known: file must be exempted
	styleSlash              // // comment
	styleHash               // # comment
	styleDash               // -- comment
	styleHTML               // <!-- comment -->
	styleCSS                // /* comment */
)

// styleFor picks the comment style from the file name.
func styleFor(name string) style {
	base := path.Base(name)
	switch base {
	case "Makefile", "makefile", "GNUmakefile", ".gitignore", ".gitattributes", ".editorconfig", "Dockerfile",
		".spdx-exempt", ".provenance-allow":
		return styleHash
	case "go.mod", "go.work":
		return styleSlash
	}
	switch strings.ToLower(path.Ext(base)) {
	case ".go", ".proto", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return styleSlash
	case ".sh", ".bash", ".yml", ".yaml", ".toml", ".mk", ".py", ".env":
		return styleHash
	case ".sql":
		return styleDash
	case ".md", ".html", ".xml", ".svg":
		return styleHTML
	case ".css":
		return styleCSS
	}
	return styleNone
}

// wrap renders text as a comment line in the given style.
func (s style) wrap(text string) string {
	switch s {
	case styleSlash:
		return "// " + text
	case styleHash:
		return "# " + text
	case styleDash:
		return "-- " + text
	case styleHTML:
		return "<!-- " + text + " -->"
	case styleCSS:
		return "/* " + text + " */"
	default:
		return text
	}
}

// unwrap strips the comment markers from a line, returning the bare text and
// whether the line was a comment in this style.
func (s style) unwrap(line string) (string, bool) {
	line = strings.TrimSpace(line)
	switch s {
	case styleSlash:
		if rest, ok := strings.CutPrefix(line, "//"); ok {
			return strings.TrimSpace(rest), true
		}
	case styleHash:
		if rest, ok := strings.CutPrefix(line, "#"); ok {
			return strings.TrimSpace(rest), true
		}
	case styleDash:
		if rest, ok := strings.CutPrefix(line, "--"); ok {
			return strings.TrimSpace(rest), true
		}
	case styleHTML:
		if rest, ok := strings.CutPrefix(line, "<!--"); ok {
			if rest, ok = strings.CutSuffix(rest, "-->"); ok {
				return strings.TrimSpace(rest), true
			}
		}
	case styleCSS:
		if rest, ok := strings.CutPrefix(line, "/*"); ok {
			if rest, ok = strings.CutSuffix(rest, "*/"); ok {
				return strings.TrimSpace(rest), true
			}
		}
	default:
	}
	return "", false
}

// Finding is one problem found in a file.
type Finding struct {
	Path    string
	Message string
}

func (f Finding) String() string { return f.Path + ": " + f.Message }

// Result is the outcome of checking one file.
type Result struct {
	Findings []Finding
	// Fixed holds the corrected content when fixing is possible and the file
	// had findings; nil otherwise.
	Fixed []byte
}

// prefixEnd returns the number of leading lines that must stay before the
// SPDX header: a shebang line or a YAML/Markdown front matter block.
func prefixEnd(lines []string, s style) int {
	if len(lines) == 0 {
		return 0
	}
	if strings.HasPrefix(lines[0], "#!") {
		return 1
	}
	if s == styleHTML && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				return i + 1
			}
		}
	}
	return 0
}

// Check verifies the SPDX header of one file's content and, when the header is
// absent or has the wrong holder or license, also computes the fixed content.
func Check(name string, content []byte) Result {
	if bytes.IndexByte(content, 0) >= 0 {
		return Result{}
	}
	s := styleFor(name)
	if s == styleNone && bytes.HasPrefix(content, []byte("#!")) {
		s = styleHash // extensionless script, e.g. a git hook
	}
	if s == styleNone {
		return Result{Findings: []Finding{{name, "unsupported file type: add the path to .spdx-exempt with a reason"}}}
	}

	text := string(content)
	lines := strings.Split(text, "\n")
	start := prefixEnd(lines, s)
	end := min(len(lines), start+headerWindow)

	copyrightIdx, licenseIdx := -1, -1
	var findings []Finding
	for i := start; i < end; i++ {
		bare, ok := s.unwrap(lines[i])
		if !ok {
			continue
		}
		switch {
		case strings.HasPrefix(bare, copyrightTag):
			copyrightIdx = i
			if bare != copyrightLine {
				findings = append(findings, Finding{name, fmt.Sprintf("line %d: copyright holder must be exactly %q", i+1, copyrightLine)})
			}
		case strings.HasPrefix(bare, licenseTag):
			licenseIdx = i
			if bare != licenseLine {
				findings = append(findings, Finding{name, fmt.Sprintf("line %d: license must be exactly %q", i+1, licenseLine)})
			}
		}
	}
	if copyrightIdx < 0 {
		findings = append(findings, Finding{name, "missing " + copyrightLine})
	}
	if licenseIdx < 0 {
		findings = append(findings, Finding{name, "missing " + licenseLine})
	}
	if len(findings) == 0 {
		return Result{}
	}

	fixed := lines
	// Correct wrong lines in place first, then add whatever is missing.
	if copyrightIdx >= 0 {
		fixed[copyrightIdx] = s.wrap(copyrightLine)
	}
	if licenseIdx >= 0 {
		fixed[licenseIdx] = s.wrap(licenseLine)
	}
	switch {
	case copyrightIdx < 0 && licenseIdx < 0:
		fixed = insertHeader(fixed, start, s, []string{s.wrap(copyrightLine), s.wrap(licenseLine)})
	case copyrightIdx < 0:
		fixed = slices.Insert(fixed, licenseIdx, s.wrap(copyrightLine))
	case licenseIdx < 0:
		fixed = slices.Insert(fixed, copyrightIdx+1, s.wrap(licenseLine))
	}
	return Result{Findings: findings, Fixed: []byte(strings.Join(fixed, "\n"))}
}

// insertHeader places the header lines at index start. Comment styles that
// do not render (everything but HTML) get a blank line after the header so it
// is never mistaken for documentation of the first declaration.
func insertHeader(lines []string, start int, s style, header []string) []string {
	out := make([]string, 0, len(lines)+len(header)+1)
	out = append(out, lines[:start]...)
	out = append(out, header...)
	if s != styleHTML && start < len(lines) && strings.TrimSpace(strings.Join(lines[start:], "")) != "" {
		out = append(out, "")
	}
	out = append(out, lines[start:]...)
	return out
}
