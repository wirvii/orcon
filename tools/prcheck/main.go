// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command prcheck runs the advisory pull request checks of the contribution
// workflow (orcon-contributing PR-6..PR-9; ADR 0081): size against the soft
// limit, the "Spec:" line, the branch name and the skill checklists.
//
// Usage:
//
//	prcheck size|spec|branch|skills|all [flags]
//
// Flags: --repo, --base, --head, --body-file, --branch, --limit, --summary,
// and --list (skills only: print the applicable skills, one per line).
//
// prcheck only warns. It exits 0 whenever it could measure, and 2 on a usage
// or git error, so a broken check is visible instead of silently green.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	exitOK    = 0
	exitUsage = 2
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)) //nolint:forbidigo // tool entry point owns the process exit code
}

// options holds the parsed command line.
type options struct {
	command  string
	repo     string
	base     string
	head     string
	bodyFile string
	branch   string
	limit    int
	summary  string
	list     bool
}

func parseOptions(args []string, stderr io.Writer) (options, int) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: prcheck size|spec|branch|skills|all [flags]")
		return options{}, exitUsage
	}
	opts := options{command: args[0]}
	flags := flag.NewFlagSet("prcheck "+opts.command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&opts.repo, "repo", ".", "repository directory")
	flags.StringVar(&opts.base, "base", "origin/main", "base revision of the pull request")
	flags.StringVar(&opts.head, "head", "HEAD", "head revision of the pull request")
	flags.StringVar(&opts.bodyFile, "body-file", "", "file with the pull request description (empty: no description)")
	flags.StringVar(&opts.branch, "branch", "", "name of the pull request branch (empty: not checked)")
	flags.IntVar(&opts.limit, "limit", DefaultLimit, "soft limit of changed lines")
	flags.StringVar(&opts.summary, "summary", os.Getenv("GITHUB_STEP_SUMMARY"), "file the summary table is appended to")
	flags.BoolVar(&opts.list, "list", false, "skills: print the applicable skills, one per line")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return options{}, exitOK
		}
		return options{}, exitUsage
	}
	switch opts.command {
	case "size", "spec", "branch", "skills", "all":
	default:
		fmt.Fprintf(stderr, "prcheck: unknown command %q\n", opts.command)
		return options{}, exitUsage
	}
	return opts, -1
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	opts, code := parseOptions(args, stderr)
	if code >= 0 {
		return code
	}

	body, haveBody, err := readBody(opts.bodyFile)
	if err != nil {
		fmt.Fprintln(stderr, "prcheck:", err)
		return exitUsage
	}

	rep := &report{limit: opts.limit}
	if opts.command == "size" || opts.command == "skills" || opts.command == "all" {
		size, err := MeasureSize(ctx, opts.repo, opts.base, opts.head)
		if err != nil {
			fmt.Fprintln(stderr, "prcheck:", err)
			return exitUsage
		}
		if opts.command == "skills" && opts.list {
			for _, skill := range ApplicableSkills(size.ChangedPaths()) {
				fmt.Fprintln(stdout, skill)
			}
			return exitOK
		}
		if opts.command != "skills" {
			rep.checkSize(size, body, haveBody)
		}
		if opts.command != "size" {
			rep.checkSkills(ApplicableSkills(size.ChangedPaths()), body, haveBody)
		}
	}
	if (opts.command == "spec" || opts.command == "all") && haveBody {
		rep.check("Spec line", CheckSpecLine(body))
	}
	if (opts.command == "branch" || opts.command == "all") && opts.branch != "" {
		rep.check("Branch name", CheckBranchName(opts.branch))
	}

	rep.print(stdout)
	if err := rep.writeSummary(opts.summary); err != nil {
		fmt.Fprintln(stderr, "prcheck:", err)
		return exitUsage
	}
	return exitOK
}

// readBody reads the pull request description. An empty path means there is no
// description to check.
func readBody(path string) (string, bool, error) {
	if path == "" {
		return "", false, nil
	}
	raw, err := os.ReadFile(path) //nolint:gosec // path is a command-line flag of a developer tool
	if err != nil {
		return "", false, fmt.Errorf("read the description: %w", err)
	}
	return string(raw), true, nil
}

// row is one line of the summary table.
type row struct {
	check  string
	result string
}

// report collects the findings of one run.
type report struct {
	limit    int
	rows     []row
	warnings []string
	notices  []string
}

func (r *report) check(name string, err error) {
	if err != nil {
		r.rows = append(r.rows, row{name, "warning: " + err.Error()})
		r.warnings = append(r.warnings, fmt.Sprintf("%s: %s", name, err.Error()))
		return
	}
	r.rows = append(r.rows, row{name, "ok"})
}

func (r *report) checkSize(size SizeResult, body string, haveBody bool) {
	result := fmt.Sprintf("%d changed lines counted (%d excluded by attribute), limit %d", size.Counted, size.Excluded, r.limit)
	if size.Counted <= r.limit {
		r.rows = append(r.rows, row{"Size", result})
		return
	}
	switch {
	case haveBody && HasSizeJustification(body):
		r.rows = append(r.rows, row{"Size", result + "; justified"})
		r.notices = append(r.notices, "Size: "+result+"; the description has a size justification")
	case haveBody:
		r.rows = append(r.rows, row{"Size", result + "; no justification"})
		r.warnings = append(r.warnings, "Size: "+result+"; add a \"## Size justification\" section or split the pull request")
	default:
		r.rows = append(r.rows, row{"Size", result + "; description not checked"})
		r.warnings = append(r.warnings, "Size: "+result+"; a pull request over the limit needs a \"## Size justification\" section")
	}
}

func (r *report) checkSkills(skills []string, body string, haveBody bool) {
	if len(skills) == 0 {
		r.rows = append(r.rows, row{"Skill checklists", "no skill applies"})
		return
	}
	list := strings.Join(skills, ", ")
	if !haveBody {
		r.rows = append(r.rows, row{"Skill checklists", "apply: " + list})
		r.notices = append(r.notices, "Skill checklists apply: "+list)
		return
	}
	missing := MissingChecklists(body, skills)
	if len(missing) == 0 {
		r.rows = append(r.rows, row{"Skill checklists", "ok (" + list + ")"})
		return
	}
	msg := "tick \"- [x] <skill>\" and paste the checklist for: " + strings.Join(missing, ", ")
	r.rows = append(r.rows, row{"Skill checklists", "warning: " + msg})
	r.warnings = append(r.warnings, "Skill checklists: "+msg)
}

// print writes the GitHub Actions annotations and a short text table.
func (r *report) print(w io.Writer) {
	for _, n := range r.notices {
		fmt.Fprintf(w, "::notice title=prcheck::%s\n", n)
	}
	for _, warn := range r.warnings {
		fmt.Fprintf(w, "::warning title=prcheck::%s\n", warn)
	}
	for _, rw := range r.rows {
		fmt.Fprintf(w, "%-18s %s\n", rw.check, rw.result)
	}
}

// writeSummary appends the Markdown table to the step summary file, if any.
func (r *report) writeSummary(path string) error {
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // GITHUB_STEP_SUMMARY or a flag
	if err != nil {
		return fmt.Errorf("open the summary: %w", err)
	}
	var b strings.Builder
	b.WriteString("### Pull request checks (advisory)\n\n| Check | Result |\n|---|---|\n")
	for _, rw := range r.rows {
		fmt.Fprintf(&b, "| %s | %s |\n", rw.check, strings.ReplaceAll(rw.result, "|", "\\|"))
	}
	if _, err := io.WriteString(f, b.String()); err != nil {
		_ = f.Close()
		return fmt.Errorf("write the summary: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close the summary: %w", err)
	}
	return nil
}
