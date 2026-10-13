// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command dbcheck runs the static checks of the Orcon database rulebook
// (.agents/skills/orcon-db/references/checks.md, SPEC-009 4.4.1) on the
// migration and query files of both storage adapters. It needs no database.
//
// Usage:
//
//	dbcheck [flags] all | <check> | sum
//
// Checks: migration-names, migration-pairs, migration-headers,
// migration-immutable, migration-expand-contract, ddl-forbidden, ddl-enum-values,
// query-select-star, query-json-path, query-limit, query-offset,
// query-keyset, query-annotations, query-soft-delete, query-hot-tag,
// query-parity, query-names, policy. "all" runs every check; "sum" appends the
// missing lines of both migrations.sum files and never rewrites a line.
//
// Findings print as "<file>:<line>: <rule> <message> (fix: <fix>)". Notes about
// conditions that do not apply (no base revision, no release tags) print as
// "note: ...". The exit status is 0 when nothing is found, 1 when something is
// found and 2 for a usage or input error. The base revision of
// migration-immutable is -base, else $DBCHECK_BASE, else the merge base of HEAD
// and origin/main.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) //nolint:forbidigo // tool entry point owns the process exit code
}

type check struct {
	name string
	fn   func(*world) []Finding
}

var checks = []check{
	{"migration-names", checkMigrationNames},
	{"migration-pairs", checkMigrationPairs},
	{"migration-headers", checkMigrationHeaders},
	{"migration-immutable", checkMigrationImmutable},
	{"migration-expand-contract", checkExpandContract},
	{"ddl-forbidden", checkForbiddenDDL},
	{"ddl-enum-values", checkEnumValues},
	{"query-select-star", checkSelectStar},
	{"query-json-path", checkJSONPath},
	{"query-limit", checkLimit},
	{"query-offset", checkOffset},
	{"query-keyset", checkKeyset},
	{"query-annotations", checkAnnotations},
	{"query-soft-delete", checkSoftDelete},
	{"query-hot-tag", checkHotTag},
	{"query-parity", checkQueryParity},
	{"query-names", checkQueryNames},
	{"policy", checkPolicy},
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("dbcheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root; the other directories default to paths below it")
	sqliteDir := flags.String("sqlite", "", "SQLite adapter directory (default <root>/internal/adapter/sqlite)")
	postgresDir := flags.String("postgres", "", "PostgreSQL adapter directory (default <root>/internal/adapter/postgres)")
	policyDir := flags.String("policy", "", "policy directory (default <root>/internal/adapter/storetest/dbrules/policy)")
	base := flags.String("base", os.Getenv("DBCHECK_BASE"), "base revision for migration-immutable (default $DBCHECK_BASE, else the merge base with origin/main; none = compare only with migrations.sum)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: dbcheck [flags] all | sum | <check>")
		fmt.Fprintln(stderr, "checks:", strings.Join(checkNames(), ", "))
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	command := flags.Arg(0)

	cfg := config{
		SQLiteDir:   orDefault(*sqliteDir, filepath.Join(*root, "internal", "adapter", "sqlite")),
		PostgresDir: orDefault(*postgresDir, filepath.Join(*root, "internal", "adapter", "postgres")),
		PolicyDir:   orDefault(*policyDir, filepath.Join(*root, "internal", "adapter", "storetest", "dbrules", "policy")),
		Base:        *base,
		Git:         execGit{},
	}
	return execute(command, cfg, stdout, stderr)
}

func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

func checkNames() []string {
	names := make([]string, 0, len(checks))
	for _, c := range checks {
		names = append(names, c.name)
	}
	return names
}

// execute runs one command against cfg and prints the result.
func execute(command string, cfg config, stdout, stderr io.Writer) int {
	w := loadWorld(cfg)
	if command == "sum" {
		added, err := runSum(w)
		if err != nil {
			fmt.Fprintln(stderr, "dbcheck:", err)
			return 2
		}
		for _, a := range added {
			fmt.Fprintln(stdout, "added", a)
		}
		fmt.Fprintf(stdout, "dbcheck: sum: %d line(s) added\n", len(added))
		return 0
	}
	var selected []check
	if command == "all" {
		selected = checks
	} else {
		for _, c := range checks {
			if c.name == command {
				selected = []check{c}
			}
		}
		if selected == nil {
			fmt.Fprintf(stderr, "dbcheck: unknown check %q (checks: %s)\n", command, strings.Join(checkNames(), ", "))
			return 2
		}
	}
	findings := append([]Finding(nil), w.loadFindings...)
	for _, c := range selected {
		findings = append(findings, c.fn(w)...)
	}
	findings = normalize(findings)
	for _, n := range w.notes {
		fmt.Fprintln(stdout, "note:", n)
	}
	for _, f := range findings {
		fmt.Fprintln(stdout, f)
	}
	if len(findings) > 0 {
		fmt.Fprintf(stderr, "dbcheck: %d finding(s)\n", len(findings))
		return 1
	}
	fmt.Fprintf(stdout, "dbcheck: %s: ok\n", command)
	return 0
}

// normalize sorts findings and drops exact repeats.
func normalize(fs []Finding) []Finding {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.Msg < b.Msg
	})
	out := fs[:0]
	for i, f := range fs {
		if i > 0 && f == fs[i-1] {
			continue
		}
		out = append(out, f)
	}
	return out
}
