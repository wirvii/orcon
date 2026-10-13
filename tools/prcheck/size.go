// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DefaultLimit is the soft limit of changed lines of a pull request
// (orcon-contributing PR-7). 400 does not warn; 401 does.
const DefaultLimit = 400

// FileChange is one changed path with its added and deleted line counts.
type FileChange struct {
	Path     string
	Lines    int
	Excluded bool
}

// SizeResult is the measured size of a pull request.
type SizeResult struct {
	// Counted is the number of changed lines that count against the limit.
	Counted int
	// Excluded is the number of changed lines left out by attribute.
	Excluded int
	// Files lists every changed path.
	Files []FileChange
}

// ChangedPaths returns every changed path, excluded or not.
func (r SizeResult) ChangedPaths() []string {
	paths := make([]string, 0, len(r.Files))
	for _, f := range r.Files {
		paths = append(paths, f.Path)
	}
	return paths
}

// MeasureSize measures the diff base...head of the repository at repo. Lines
// are added plus deleted; binary files count zero; paths whose .gitattributes
// set pr-size-exclude or linguist-generated are excluded.
func MeasureSize(ctx context.Context, repo, base, head string) (SizeResult, error) {
	raw, err := git(ctx, repo, nil, "diff", "--numstat", "-z", "-M", base+"..."+head)
	if err != nil {
		return SizeResult{}, err
	}
	files, err := parseNumstat(raw)
	if err != nil {
		return SizeResult{}, err
	}
	excluded, err := excludedPaths(ctx, repo, files)
	if err != nil {
		return SizeResult{}, err
	}
	var res SizeResult
	for i := range files {
		files[i].Excluded = excluded[files[i].Path]
		if files[i].Excluded {
			res.Excluded += files[i].Lines
		} else {
			res.Counted += files[i].Lines
		}
	}
	res.Files = files
	return res, nil
}

// parseNumstat parses `git diff --numstat -z -M`. A normal entry is
// "added\tdeleted\tpath\0"; a rename is "added\tdeleted\t\0old\0new\0". Binary
// files report "-" and count zero.
func parseNumstat(raw []byte) ([]FileChange, error) {
	var files []FileChange
	fields := bytes.Split(raw, []byte{0})
	for i := 0; i < len(fields); i++ {
		entry := string(fields[i])
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "\t", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("parse numstat: unexpected entry %q", entry)
		}
		lines, err := countLines(parts[0], parts[1])
		if err != nil {
			return nil, err
		}
		path := parts[2]
		if path == "" {
			// Rename: the next two fields are the old and the new path.
			if i+2 >= len(fields) {
				return nil, fmt.Errorf("parse numstat: truncated rename entry %q", entry)
			}
			path = string(fields[i+2])
			i += 2
		}
		files = append(files, FileChange{Path: path, Lines: lines})
	}
	return files, nil
}

func countLines(added, deleted string) (int, error) {
	if added == "-" || deleted == "-" {
		return 0, nil // binary file
	}
	a, err := strconv.Atoi(added)
	if err != nil {
		return 0, fmt.Errorf("parse numstat: added lines %q: %w", added, err)
	}
	d, err := strconv.Atoi(deleted)
	if err != nil {
		return 0, fmt.Errorf("parse numstat: deleted lines %q: %w", deleted, err)
	}
	return a + d, nil
}

// excludedPaths asks git which paths carry pr-size-exclude or
// linguist-generated; .gitattributes is the single source of the exclusions.
func excludedPaths(ctx context.Context, repo string, files []FileChange) (map[string]bool, error) {
	excluded := make(map[string]bool, len(files))
	if len(files) == 0 {
		return excluded, nil
	}
	var in bytes.Buffer
	for _, f := range files {
		in.WriteString(f.Path)
		in.WriteByte(0)
	}
	out, err := git(ctx, repo, in.Bytes(), "check-attr", "--stdin", "-z", "pr-size-exclude", "linguist-generated")
	if err != nil {
		return nil, err
	}
	// Output is a stream of "path\0attribute\0value\0" triples.
	fields := bytes.Split(out, []byte{0})
	for i := 0; i+2 < len(fields); i += 3 {
		path, value := string(fields[i]), string(fields[i+2])
		switch value {
		case "unspecified", "unset", "false", "":
		default:
			excluded[path] = true
		}
	}
	return excluded, nil
}

var justificationHeading = regexp.MustCompile(`(?m)^##[ \t]+Size justification[ \t]*$`)

// minJustification is the least number of characters of a justification.
const minJustification = 20

// HasSizeJustification reports whether the pull request body has a "## Size
// justification" section with at least one line that is not an HTML comment
// and has at least 20 characters, before the next heading.
func HasSizeJustification(body string) bool {
	loc := justificationHeading.FindStringIndex(body)
	if loc == nil {
		return false
	}
	inComment := false
	for _, line := range strings.Split(body[loc[1]:], "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") && !inComment {
			return false
		}
		text, stillInComment := stripComments(line, inComment)
		inComment = stillInComment
		if len(strings.TrimSpace(text)) >= minJustification {
			return true
		}
	}
	return false
}

// stripComments removes HTML comments from one line. inComment tells whether
// the line starts inside a comment; the second result tells whether it ends
// inside one.
func stripComments(line string, inComment bool) (string, bool) {
	var b strings.Builder
	rest := line
	for rest != "" {
		if inComment {
			end := strings.Index(rest, "-->")
			if end < 0 {
				return b.String(), true
			}
			rest = rest[end+len("-->"):]
			inComment = false
			continue
		}
		start := strings.Index(rest, "<!--")
		if start < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:start])
		rest = rest[start+len("<!--"):]
		inComment = true
	}
	return b.String(), inComment
}
