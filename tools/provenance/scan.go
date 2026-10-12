// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	"github.com/wirvii/orcon/tools/internal/glob"
	"github.com/wirvii/orcon/tools/provenance/configpaths"
)

// Kind says what a piece of text is, because the rules differ.
type Kind int

const (
	// KindFile is the content of a tracked file.
	KindFile Kind = iota
	// KindCommit is a commit message; "#" comment lines and everything after
	// the scissors line are ignored, as git would strip them.
	KindCommit
	// KindPR is pull request text (title or body); nothing is stripped.
	KindPR
)

// Class is the severity class of a finding.
type Class byte

const (
	// ClassA is an AI authorship mark. It is never allowed.
	ClassA Class = 'A'
	// ClassB is the name of an assistant or its vendor. It is allowed in
	// product AI feature paths and under product commit scopes.
	ClassB Class = 'B'
)

// Finding is one match of the provenance rules.
type Finding struct {
	Line  int // 1-based line within the scanned text
	Class Class
	Term  string
	Why   string
}

func (f Finding) String() string {
	return fmt.Sprintf("line %d: class %c: %q (%s): %s", f.Line, f.Class, f.Term, f.Why, fixHint)
}

// Options tune a scan.
type Options struct {
	// AllowClassB lets class B terms through: the file is under a product AI
	// path, or the commit/PR subject uses a product scope. Class A is never
	// affected, and trailer lines never benefit from it.
	AllowClassB bool
}

const scissors = "# ------------------------ >8 ------------------------"

// Scan applies the provenance rules to text and returns every finding.
func Scan(kind Kind, text string, opts Options) []Finding {
	lines := strings.Split(text, "\n")
	message := kind != KindFile

	allowB := opts.AllowClassB
	trailers := map[int]bool{}
	if kind == KindCommit {
		lines = stripCommitComments(lines)
		allowB = allowB || ProductScope(firstLine(lines))
	}
	if message {
		trailers = trailerLines(lines)
	}

	var findings []Finding
	for i, line := range lines {
		n := i + 1
		a := classAFindings(n, line, message)
		findings = append(findings, a...)
		if len(a) > 0 {
			// The class A finding already explains the line; do not also
			// report the vendor name inside it.
			continue
		}
		if allowB && !trailers[i] {
			continue
		}
		for _, m := range classB.FindAllString(line, -1) {
			findings = append(findings, Finding{Line: n, Class: ClassB, Term: m, Why: "assistant or vendor name outside a product AI path or scope"})
		}
	}
	return findings
}

// classAFindings returns the class A findings of one line.
func classAFindings(n int, line string, message bool) []Finding {
	var out []Finding
	add := func(term, why string) { out = append(out, Finding{Line: n, Class: ClassA, Term: term, Why: why}) }

	if m := coAuthoredBy.FindStringSubmatch(line); m != nil {
		if classB.MatchString(m[1]) || vendorMail.MatchString(m[1]) {
			add(strings.TrimSpace(line), "Co-Authored-By trailer naming an AI")
		}
	}
	if sessionTrailer.MatchString(line) {
		add(strings.TrimSpace(line), "assistant session trailer")
	}
	if message && generatedLead.MatchString(line) {
		add(strings.TrimSpace(line), `"Generated with" line`)
	}
	if m := generatedByAI.FindString(line); m != "" {
		add(m, "AI generation credit")
	}
	if m := aiMarks.FindString(line); m != "" {
		add(m, "AI authorship mark")
	}
	if m := robotGenerated.FindString(line); m != "" {
		add(m, "robot emoji generation credit")
	}
	// One finding per line is enough to explain it.
	if len(out) > 1 {
		out = out[:1]
	}
	return out
}

func stripCommitComments(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if l == scissors {
			break
		}
		if strings.HasPrefix(l, "#") {
			// Keep numbering stable for the reader: a stripped comment is an
			// empty line, which also cannot create a trailer or a subject.
			out = append(out, "")
			continue
		}
		out = append(out, l)
	}
	return out
}

func firstLine(lines []string) string {
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			return l
		}
	}
	return ""
}

// ProductScope reports whether a Conventional Commits subject uses one of the
// product scopes (ai, llm, mcp).
func ProductScope(subject string) bool {
	m := conventionalSubject.FindStringSubmatch(subject)
	if m == nil {
		return false
	}
	for _, s := range strings.Split(m[1], ",") {
		if productScopes[strings.ToLower(strings.TrimSpace(s))] {
			return true
		}
	}
	return false
}

// trailerLines returns the indexes of trailer lines: "Token: value" lines in
// the final paragraph of a message that has a body.
func trailerLines(lines []string) map[int]bool {
	out := map[int]bool{}
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	start := end
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	if start == 0 {
		// The last paragraph is the subject paragraph: no body, no trailers.
		return out
	}
	for i := start; i < end; i++ {
		if trailerLine.MatchString(lines[i]) {
			out[i] = true
		}
	}
	return out
}

// IsExempt reports whether a tracked path is outside the check: the exempt
// configuration paths of owner decision D-O4 plus tools/provenance/**, the
// technical exemption for the term list and its test messages.
func IsExempt(name string) bool { return glob.MatchAny(configpaths.Provenance, name) }

// IsBinary reports whether content looks binary (contains a NUL byte).
func IsBinary(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return true
		}
	}
	return false
}
