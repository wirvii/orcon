// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command licensecheck decides whether the licenses of a module's linked
// dependencies are acceptable (SPEC-008 CFG-4, dependencies.md DEP-2).
//
// It reads the tab-separated report of go-licenses on standard input, one
// "module<TAB>version<TAB>license" row per dependency, and fails for every row
// whose license is not on the allow list unless the exact module@version has a
// well-formed, owner-approved line in tools/licenses/exceptions.txt. Licenses
// that Orcon never ships (GPL, AGPL, LGPL, SSPL, BUSL, Elastic, Commons Clause
// and other source-available terms) cannot be excepted. A malformed exceptions
// line fails the check.
//
// Usage:
//
//	go-licenses report ./... --template rows.tmpl | licensecheck [-exceptions tools/licenses/exceptions.txt] [-ignore module]...
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) //nolint:forbidigo // tool entry point owns the process exit code
}

// allowed are the licenses that may be linked into Orcon without approval.
var allowed = map[string]bool{
	"Apache-2.0":   true,
	"MIT":          true,
	"BSD-2-Clause": true,
	"BSD-3-Clause": true,
	"ISC":          true,
}

// neverExcepted are license families no exception can admit (DEP-2b.3).
var neverExcepted = []string{
	"GPL", "AGPL", "LGPL", "SSPL", "BUSL", "Elastic", "Commons-Clause", "PolyForm", "CC-BY-NC",
}

// exceptionLine is: <module>@<version>  <SPDX id>  <reason>  <YYYY-MM-DD>  approved-by:<owner>
var exceptionLine = regexp.MustCompile(`^(\S+)@(\S+) {2,}(\S+) {2,}(\S.*?) {2,}(\d{4}-\d{2}-\d{2}) {2,}approved-by:(\S+)$`)

type exception struct {
	license string
	line    int
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("licensecheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	exceptionsFile := flags.String("exceptions", "tools/licenses/exceptions.txt", "file listing owner-approved per-module exceptions")
	var ignore stringList
	flags.Var(&ignore, "ignore", "module path to skip, with the packages below it (the module being checked); may be repeated")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	f, err := os.Open(*exceptionsFile) //nolint:gosec // path comes from the command line of a developer tool
	if err != nil {
		fmt.Fprintln(stderr, "licensecheck:", err)
		return 2
	}
	defer f.Close()
	exceptions, problems := parseExceptions(f)
	for _, p := range problems {
		fmt.Fprintf(stderr, "licensecheck: %s:%s\n", *exceptionsFile, p)
	}
	if len(problems) > 0 {
		return 1
	}

	failures, err := check(stdin, exceptions, ignore)
	if err != nil {
		fmt.Fprintln(stderr, "licensecheck:", err)
		return 2
	}
	for _, msg := range failures {
		fmt.Fprintln(stderr, "licensecheck:", msg)
	}
	if len(failures) > 0 {
		fmt.Fprintf(stderr, "licensecheck: %d dependency license(s) not allowed; see dependencies.md (License exceptions)\n", len(failures))
		return 1
	}
	return 0
}

// parseExceptions reads the exceptions file. Blank lines and lines starting
// with "#" are ignored. Every other line must match the documented format.
func parseExceptions(r io.Reader) (map[string]exception, []string) {
	out := map[string]exception{}
	var problems []string
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimRight(sc.Text(), " \t\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		m := exceptionLine.FindStringSubmatch(line)
		if m == nil {
			problems = append(problems, fmt.Sprintf("%d: malformed exception (want \"<module>@<version>  <SPDX id>  <reason>  <YYYY-MM-DD>  approved-by:<owner>\"): %q", n, line))
			continue
		}
		module, version, license, date := m[1], m[2], m[3], m[5]
		if _, err := time.Parse("2006-01-02", date); err != nil {
			problems = append(problems, fmt.Sprintf("%d: %q is not a valid date", n, date))
			continue
		}
		if forbidden(license) {
			problems = append(problems, fmt.Sprintf("%d: license %s cannot be admitted by an exception", n, license))
			continue
		}
		key := module + "@" + version
		if _, dup := out[key]; dup {
			problems = append(problems, fmt.Sprintf("%d: duplicate exception for %s", n, key))
			continue
		}
		out[key] = exception{license: license, line: n}
	}
	if err := sc.Err(); err != nil {
		problems = append(problems, "read: "+err.Error())
	}
	return out, problems
}

// forbidden reports whether the license belongs to a family that no exception
// can admit.
func forbidden(license string) bool {
	upper := strings.ToUpper(license)
	for _, family := range neverExcepted {
		if strings.HasPrefix(upper, strings.ToUpper(family)) {
			return true
		}
	}
	return false
}

// ignored reports whether the row belongs to one of the ignored modules: the
// module itself, or a package below it (go-licenses lists the packages of the
// module being checked, which have no version of their own).
func ignored(module string, ignore []string) bool {
	for _, m := range ignore {
		if module == m || strings.HasPrefix(module, m+"/") {
			return true
		}
	}
	return false
}

// check reads the report rows and returns one message per disallowed
// dependency.
func check(r io.Reader, exceptions map[string]exception, ignore []string) ([]string, error) {
	seen := map[string]bool{}
	var failures []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		row := strings.TrimSpace(sc.Text())
		if row == "" {
			continue
		}
		cols := strings.Split(row, "\t")
		if len(cols) != 3 {
			return nil, fmt.Errorf("unexpected report row %q (want module, version, license separated by tabs)", row)
		}
		module, version, license := cols[0], cols[1], cols[2]
		if ignored(module, ignore) || seen[module+"@"+version] {
			continue
		}
		seen[module+"@"+version] = true
		if allowed[license] {
			continue
		}
		if forbidden(license) {
			failures = append(failures, fmt.Sprintf("%s@%s is %s, which Orcon never links", module, version, license))
			continue
		}
		if ex, ok := exceptions[module+"@"+version]; ok {
			if ex.license == license {
				continue
			}
			failures = append(failures, fmt.Sprintf("%s@%s is %s but its exception (line %d) names %s", module, version, license, ex.line, ex.license))
			continue
		}
		failures = append(failures, fmt.Sprintf("%s@%s is %s, which is not allowed and has no approved exception", module, version, license))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}
	sort.Strings(failures)
	return failures, nil
}
