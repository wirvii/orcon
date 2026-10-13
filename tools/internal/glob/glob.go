// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package glob matches slash-separated repository paths against glob patterns
// that support "**" (any number of path segments, including none) on top of
// the segment syntax of path.Match.
package glob

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"strings"
)

// Match reports whether name matches pattern. Both use "/" separators and are
// relative to the repository root. A pattern ending in "/**" matches everything
// beneath the directory; "**" as a segment matches zero or more segments.
func Match(pattern, name string) bool {
	return match(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func match(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			rest := pat[1:]
			if len(rest) == 0 {
				return true
			}
			for i := 0; i <= len(name); i++ {
				if match(rest, name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		ok, err := path.Match(pat[0], name[0])
		if err != nil || !ok {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}

// ParseList reads a glob list: one "glob  # reason" per line, with blank lines
// and whole-line "#" comments ignored. The reason is mandatory so that every
// exemption can be reviewed. name is used only in error messages.
func ParseList(r io.Reader, name string) ([]string, error) {
	var globs []string
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pattern, reason, ok := strings.Cut(line, "#")
		pattern = strings.TrimSpace(pattern)
		if !ok || pattern == "" || strings.TrimSpace(reason) == "" {
			return nil, fmt.Errorf("%s:%d: want \"glob  # reason\" with a non-empty reason", name, n)
		}
		globs = append(globs, pattern)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return globs, nil
}

// MatchAny reports whether name matches at least one of patterns.
func MatchAny(patterns []string, name string) bool {
	for _, p := range patterns {
		if Match(p, name) {
			return true
		}
	}
	return false
}
