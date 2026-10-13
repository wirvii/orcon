// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package glob_test

import (
	"strings"
	"testing"

	"github.com/wirvii/orcon/tools/internal/glob"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern, name string
		want          bool
	}{
		{".cfg/**", ".cfg/settings.json", true},
		{".cfg/**", ".cfg/agents/a.md", true},
		{".cfg/**", "docs/.cfg/x", false},
		{"**/NOTES.md", "NOTES.md", true},
		{"**/NOTES.md", "apps/web/NOTES.md", true},
		{"**/NOTES.md", "apps/web/NOTES.md.bak", false},
		{"docs/adr/006[0-9]-*.md", "docs/adr/0061-ai-agents.md", true},
		{"docs/adr/006[0-9]-*.md", "docs/adr/0074-x.md", false},
		{"docs/reference/ai-prices*", "docs/reference/ai-prices-2026.md", true},
		{"internal/ai/**", "internal/ai/cost/cost.go", true},
		{"internal/ai/**", "internal/air/x.go", false},
		{"a/**/c", "a/c", true},
		{"a/**/c", "a/b/b/c", true},
		{"a/**/c", "a/b/d", false},
		{"README.md", "README.md", true},
		{"README.md", "docs/README.md", false},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+" vs "+tt.name, func(t *testing.T) {
			if got := glob.Match(tt.pattern, tt.name); got != tt.want {
				t.Fatalf("Match(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
			}
		})
	}
}

func TestParseList(t *testing.T) {
	if _, err := glob.ParseList(strings.NewReader("apps/web/**\n"), "x"); err == nil {
		t.Fatal("a glob without a reason must be rejected")
	}
	if _, err := glob.ParseList(strings.NewReader("apps/web/**  #   \n"), "x"); err == nil {
		t.Fatal("a blank reason must be rejected")
	}
	globs, err := glob.ParseList(strings.NewReader("# header\n\na/**  # why\n"), "x")
	if err != nil || len(globs) != 1 || globs[0] != "a/**" {
		t.Fatalf("globs = %v, err = %v", globs, err)
	}
}

func TestMatchAny(t *testing.T) {
	if !glob.MatchAny([]string{"x/**", "y/*.go"}, "y/a.go") {
		t.Fatal("expected a match")
	}
	if glob.MatchAny(nil, "y/a.go") {
		t.Fatal("empty pattern list must not match")
	}
}
