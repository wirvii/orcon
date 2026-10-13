// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Default gate parameters (SPEC-008 CFG-7).
const (
	defaultThreshold = 10.0 // percent
	defaultAlpha     = 0.05
)

// JustificationHeading is the pull request section that must be non-empty
// when a regression is accepted.
const JustificationHeading = "### Performance regression justification"

// gatedUnits are the benchstat units the gate watches.
var gatedUnits = map[string]bool{"sec/op": true, "B/op": true, "allocs/op": true}

// Row is one benchmark comparison from benchstat's CSV output.
type Row struct {
	Name     string
	Unit     string
	Delta    float64 // percent change versus the base; valid when HasDelta
	HasDelta bool    // false for "~" (benchstat found no significant change)
	P        float64 // p-value; 1 when absent
}

func (r Row) String() string {
	if !r.HasDelta {
		return fmt.Sprintf("%s %s: no significant change (p=%.3f)", r.Name, r.Unit, r.P)
	}
	return fmt.Sprintf("%s %s: %+.2f%% (p=%.3f)", r.Name, r.Unit, r.Delta, r.P)
}

// Options tune the gate.
type Options struct {
	Threshold float64 // percent; a change strictly above it can fail
	Alpha     float64 // significance level; p must be strictly below it
	// Accepted is true when the pull request carries the
	// performance-regression-accepted label.
	Accepted bool
	// Body is the pull request description, searched for the justification.
	Body string
}

// Result is the gate's verdict.
type Result struct {
	Pass        bool
	Regressions []Row
	Message     string
}

var pValue = regexp.MustCompile(`p\s*=\s*([0-9.eE+-]+)`)

// ParseCSV reads benchstat's "-format csv" output. Each table starts with a
// header row whose second cell is the unit; rows below it are benchmarks until
// a blank line. Rows for units the gate does not watch, and the geomean row,
// are skipped.
func ParseCSV(r io.Reader) ([]Row, error) {
	var (
		rows []Row
		unit string
	)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			unit = ""
			continue
		}
		cells := strings.Split(line, ",")
		if len(cells) < 7 {
			continue // preamble such as "goos: linux", or a malformed line
		}
		if cells[0] == "" {
			// File-name row or unit header row.
			if gatedUnits[cells[1]] {
				unit = cells[1]
			} else if strings.Contains(strings.Join(cells, ","), "/op") {
				unit = "" // another unit (MB/s, custom metric): not gated
			}
			continue
		}
		if unit == "" || cells[0] == "geomean" {
			continue
		}
		row, err := parseRow(cells, unit)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read benchstat csv: %w", err)
	}
	return rows, nil
}

func parseRow(cells []string, unit string) (Row, error) {
	row := Row{Name: cells[0], Unit: unit, P: 1}
	delta := strings.TrimSpace(cells[5])
	if m := pValue.FindStringSubmatch(cells[6]); m != nil {
		p, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return Row{}, fmt.Errorf("benchmark %s: bad p-value %q: %w", row.Name, cells[6], err)
		}
		row.P = p
	}
	switch {
	case delta == "~" || delta == "":
		return row, nil
	case strings.HasSuffix(delta, "%"):
		d, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimPrefix(delta, "+"), "%"), 64)
		if err != nil {
			return Row{}, fmt.Errorf("benchmark %s: bad delta %q: %w", row.Name, delta, err)
		}
		row.Delta, row.HasDelta = d, true
		return row, nil
	default:
		return Row{}, fmt.Errorf("benchmark %s: unrecognized delta %q", row.Name, delta)
	}
}

// Gate decides whether the rows pass. A regression is a gated metric that got
// worse by more than the threshold with a p-value below alpha.
func Gate(rows []Row, opts Options) Result {
	if opts.Threshold == 0 {
		opts.Threshold = defaultThreshold
	}
	if opts.Alpha == 0 {
		opts.Alpha = defaultAlpha
	}

	var regressions []Row
	for _, r := range rows {
		if r.HasDelta && r.Delta > opts.Threshold && r.P < opts.Alpha {
			regressions = append(regressions, r)
		}
	}
	if len(regressions) == 0 {
		return Result{Pass: true, Message: fmt.Sprintf("no metric regressed by more than %.0f%% with p<%.2f", opts.Threshold, opts.Alpha)}
	}

	if !opts.Accepted {
		return Result{
			Regressions: regressions,
			Message:     "regression over the threshold; fix it, or add the performance-regression-accepted label and a \"" + JustificationHeading + "\" section",
		}
	}
	if Justification(opts.Body) == "" {
		return Result{
			Regressions: regressions,
			Message:     "the performance-regression-accepted label needs a non-empty \"" + JustificationHeading + "\" section in the pull request description",
		}
	}
	return Result{Pass: true, Regressions: regressions, Message: "regression accepted with a written justification"}
}

var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)

// Justification returns the text under the justification heading of a pull
// request body, up to the next heading, with HTML comments (template hints)
// removed. It is empty when the section is missing or blank.
func Justification(body string) string {
	var (
		in  bool
		buf strings.Builder
	)
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, JustificationHeading) {
			in = true
			continue
		}
		if in && strings.HasPrefix(trimmed, "#") {
			break
		}
		if in {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	return strings.TrimSpace(htmlComment.ReplaceAllString(buf.String(), ""))
}

// errNoRows is returned when the input holds no benchmark comparison at all,
// which would otherwise let a broken benchmark run pass silently.
var errNoRows = errors.New("no benchmark rows found in the benchstat csv")
