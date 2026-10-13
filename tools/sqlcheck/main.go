// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command sqlcheck is wall 2(b) of organization isolation (ADR-0046 section 3,
// SPEC-004 R-ISO-02, orcon-db TI-3). It reads the SQL files and needs no
// database.
//
// Usage:
//
//	sqlcheck [-root dir] [-policy dir] <adapter dir>...
//
// Each adapter directory (internal/adapter/sqlite, internal/adapter/postgres)
// holds migrations/ and queries/. The checks:
//
//   - SQL-1: no .sql file anywhere in the repository outside the data-access
//     layer (internal/adapter/sqlite, internal/adapter/postgres), the contract
//     suites (internal/adapter/storetest), the tools module (its test data) and
//     the skills (templates). The Go side of this rule is the orgscope analyzer
//     of tools/orconvet.
//   - TI-2: every query carries -- orcon:scope org|instance|engine.
//   - TI-3, scope org: every organization-scoped table the statement reads or
//     writes is restricted by org_id = <parameter> in a WHERE or JOIN ON clause
//     (or, through equal org_id columns, by a table that is); an INSERT sets
//     org_id from a parameter. A comparison under OR or NOT does not count.
//   - TI-3, scope instance: the statement references no organization-scoped
//     table.
//   - TI-3, scope engine: the query is listed in the reviewed
//     engine_allowlist.yaml of the policy directory.
//
// An organization-scoped table is a table the migrations create that is not
// listed in the policy file instance_tables.txt (TI-1).
//
// Findings print as "<file>:<line>: <rule> <message>". The exit status is 0
// when nothing is found, 1 when something is found and 2 for a usage or input
// error.
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

// Finding is one violation.
type Finding struct {
	File string
	Line int
	Rule string
	Msg  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s %s", f.File, f.Line, f.Rule, f.Msg)
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("sqlcheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "repository root for the SQL-1 scan (default: derived from the first adapter directory)")
	policy := flags.String("policy", "", "policy directory (default <root>/internal/adapter/storetest/dbrules/policy)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: sqlcheck [-root dir] [-policy dir] <adapter dir>...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() == 0 {
		flags.Usage()
		return 2
	}
	dirs := flags.Args()
	repo := *root
	if repo == "" {
		var err error
		if repo, err = deriveRoot(dirs[0]); err != nil {
			fmt.Fprintln(stderr, "sqlcheck:", err)
			return 2
		}
	}
	policyDir := *policy
	if policyDir == "" {
		policyDir = filepath.Join(repo, "internal", "adapter", "storetest", "dbrules", "policy")
	}

	findings, err := check(repo, policyDir, dirs)
	if err != nil {
		fmt.Fprintln(stderr, "sqlcheck:", err)
		return 2
	}
	for _, f := range findings {
		fmt.Fprintln(stdout, f)
	}
	if len(findings) > 0 {
		fmt.Fprintf(stderr, "sqlcheck: %d finding(s)\n", len(findings))
		return 1
	}
	fmt.Fprintln(stdout, "sqlcheck: ok")
	return 0
}

// deriveRoot walks up from an adapter directory to the directory above internal/.
func deriveRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	parts := strings.Split(filepath.ToSlash(abs), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "internal" {
			return filepath.FromSlash(strings.Join(parts[:i], "/")), nil
		}
	}
	return "", fmt.Errorf("cannot derive the repository root from %s; pass -root", dir)
}

// check runs every check and returns the sorted findings.
func check(repo, policyDir string, adapterDirs []string) ([]Finding, error) {
	instance, err := readNames(filepath.Join(policyDir, "instance_tables.txt"))
	if err != nil {
		return nil, err
	}
	engine, engineFindings, err := readEngineAllowlist(filepath.Join(policyDir, "engine_allowlist.yaml"))
	if err != nil {
		return nil, err
	}
	findings := append([]Finding(nil), engineFindings...)

	outside, err := checkOutsideSQL(repo)
	if err != nil {
		return nil, err
	}
	findings = append(findings, outside...)

	used := map[string]bool{}
	for _, dir := range adapterDirs {
		fs, err := checkAdapter(dir, instance, engine, used)
		if err != nil {
			return nil, err
		}
		findings = append(findings, fs...)
	}
	for _, e := range engine.entries {
		if !used[e.key()] && engine.known(e.dialect, adapterDirs) {
			findings = append(findings, Finding{File: engine.path, Line: e.line, Rule: "TI-3",
				Msg: fmt.Sprintf("allowlist entry %s %s matches no query; remove it", e.dialect, e.query)})
		}
	}
	return normalize(findings), nil
}

// checkAdapter checks the queries of one adapter directory.
func checkAdapter(dir string, instance map[string]bool, engine *allowlist, used map[string]bool) ([]Finding, error) {
	dialect := filepath.Base(dir)
	scoped, err := scopedTablesOf(filepath.Join(dir, "migrations"), instance)
	if err != nil {
		return nil, err
	}
	queries, err := readQueries(filepath.Join(dir, "queries"))
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, q := range queries {
		out = append(out, checkQuery(q, dialect, scoped, engine, used)...)
	}
	return out, nil
}

// checkQuery applies the scope rules to one query.
func checkQuery(q query, dialect string, scoped scopedTables, engine *allowlist, used map[string]bool) []Finding {
	finding := func(line int, rule, msg string) Finding {
		if line == 0 {
			line = q.line
		}
		return Finding{File: q.file, Line: line, Rule: rule, Msg: fmt.Sprintf("query %s: %s", q.name, msg)}
	}
	switch q.scope {
	case "":
		return []Finding{finding(0, "TI-2", "missing -- orcon:scope org|instance|engine")}
	case "engine":
		entry := engine.find(dialect, q.name)
		if entry == nil {
			return []Finding{finding(0, "TI-3", fmt.Sprintf("scope engine, but %s %s is not listed in engine_allowlist.yaml (reviewed by CODEOWNERS)", dialect, q.name))}
		}
		used[entry.key()] = true
		return nil
	case "org", "instance":
	default:
		return []Finding{finding(0, "TI-2", fmt.Sprintf("scope %q must be org, instance or engine", q.scope))}
	}

	toks, err := lex(q.body, q.bodyLine)
	if err != nil {
		return []Finding{finding(0, "TI-3", fmt.Sprintf("cannot read the statement: %v", err))}
	}
	a, err := analyze(toks)
	if err != nil {
		return []Finding{finding(0, "TI-3", fmt.Sprintf("cannot read the statement: %v", err))}
	}
	var problems []problem
	if q.scope == "org" {
		problems = checkOrgStatement(a, scoped)
	} else {
		problems = checkInstanceStatement(a, scoped)
	}
	sortProblems(problems)
	out := make([]Finding, 0, len(problems))
	for _, p := range problems {
		out = append(out, finding(p.line, "TI-3", p.msg))
	}
	return out
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
