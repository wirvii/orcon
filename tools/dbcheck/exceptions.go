// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"
)

// exception is one owner-approved line of policy/exceptions.txt:
//
//	<rule-id>  <dialect>:<object>  <reason>  <YYYY-MM-DD>  approved-by:<owner>
type exception struct {
	rule     string
	dialect  string
	object   string
	reason   string
	expires  time.Time
	approver string
	line     int
}

var exceptionLineRE = regexp.MustCompile(`^([A-Z]{2}-\d+) {2,}(sqlite|postgres):(\S+) {2,}(\S.*?) {2,}(\d{4}-\d{2}-\d{2}) {2,}approved-by:(\S+)$`)

// exceptions is the parsed exceptions file.
type exceptions struct {
	list []exception
}

// loadExceptions parses exceptions.txt. now decides which lines are past
// their date; a line is valid through its date.
func loadExceptions(dir string, now time.Time) (*exceptions, []Finding) {
	lines, fs := readDataLines(dir, fileExceptions)
	path := filepath.Join(dir, fileExceptions)
	ex := &exceptions{}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	for _, l := range lines {
		m := exceptionLineRE.FindStringSubmatch(l.value)
		if m == nil {
			fs = append(fs, Finding{File: path, Line: l.line, Rule: "CK-20",
				Msg: fmt.Sprintf("malformed exception %q: expected \"<rule-id>  <dialect>:<object>  <reason>  <YYYY-MM-DD>  approved-by:<owner>\"", l.value),
				Fix: "write all five fields separated by two or more spaces; an exception needs approved-by: (owner approval)"})
			continue
		}
		until, err := time.Parse("2006-01-02", m[5])
		if err != nil {
			fs = append(fs, Finding{File: path, Line: l.line, Rule: "CK-20", Msg: fmt.Sprintf("exception date %q is not a valid date", m[5]), Fix: "use YYYY-MM-DD"})
			continue
		}
		if until.Before(today) {
			fs = append(fs, Finding{File: path, Line: l.line, Rule: "CK-21", Msg: fmt.Sprintf("exception for %s %s:%s expired on %s", m[1], m[2], m[3], m[5]), Fix: "remove the exception or renew it with a new owner-approved date"})
			continue
		}
		ex.list = append(ex.list, exception{rule: m[1], dialect: m[2], object: m[3], reason: m[4], expires: until, approver: m[6], line: l.line})
	}
	return ex, fs
}

// has reports whether an unexpired approved exception exists for the rule
// and the object in the dialect.
func (e *exceptions) has(rule, dialect, object string) bool {
	if e == nil {
		return false
	}
	for _, x := range e.list {
		if x.rule == rule && x.dialect == dialect && x.object == object {
			return true
		}
	}
	return false
}
