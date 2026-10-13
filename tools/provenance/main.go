// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command provenance keeps AI authorship marks and assistant names out of
// Orcon's files, commit messages and pull request text (SPEC-008 4.11).
//
// Usage:
//
//	provenance files [-allow .provenance-allow] [path...]
//	provenance msg <message-file>
//	provenance range <base> <head> [--pr-title T] [--pr-body-file F]
//
// files scans tracked files (or the given paths); msg checks one commit
// message (the commit-msg hook); range checks every commit of base..head and
// optionally the pull request title and body. It never rewrites history.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/wirvii/orcon/tools/internal/glob"
)

const zeroSHA = "0000000000000000000000000000000000000000"

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)) //nolint:forbidigo // tool entry point owns the process exit code
}

const usage = `usage:
  provenance files [-allow .provenance-allow] [path...]
  provenance msg <message-file>
  provenance range <base> <head> [--pr-title T] [--pr-body-file F]`

// run returns 0 when clean, 1 when the check found something, 2 on usage or
// internal errors.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	var (
		found int
		err   error
	)
	switch args[0] {
	case "files":
		found, err = runFiles(ctx, args[1:], stdout)
	case "msg":
		found, err = runMsg(args[1:], stdout)
	case "range":
		found, err = runRange(ctx, args[1:], stdout)
	case "-h", "-help", "--help":
		fmt.Fprintln(stdout, usage)
		return 0
	default:
		fmt.Fprintln(stderr, usage)
		return 2
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "provenance:", err)
		return 2
	}
	if found > 0 {
		fmt.Fprintf(stderr, "provenance: %d finding(s); %s\n", found, fixHint)
		return 1
	}
	return 0
}

func runFiles(ctx context.Context, args []string, stdout io.Writer) (int, error) {
	flags := flag.NewFlagSet("files", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	allowFile := flags.String("allow", ".provenance-allow", "file listing product AI paths where class B terms are allowed")
	if err := flags.Parse(args); err != nil {
		return 0, fmt.Errorf("parse flags: %w", err)
	}
	allow, err := loadAllow(*allowFile)
	if err != nil {
		return 0, err
	}
	paths := flags.Args()
	if len(paths) == 0 {
		paths, err = trackedFiles(ctx)
		if err != nil {
			return 0, err
		}
	}
	found := 0
	for _, p := range paths {
		if IsExempt(p) {
			continue
		}
		content, err := os.ReadFile(p) //nolint:gosec // paths come from git ls-files or the command line
		if errors.Is(err, os.ErrNotExist) {
			continue // listed but deleted in the working tree
		}
		if err != nil {
			return 0, fmt.Errorf("read %s: %w", p, err)
		}
		if IsBinary(content) {
			continue
		}
		for _, f := range Scan(KindFile, string(content), Options{AllowClassB: glob.MatchAny(allow, p)}) {
			fmt.Fprintf(stdout, "%s:%s\n", p, f)
			found++
		}
	}
	return found, nil
}

func runMsg(args []string, stdout io.Writer) (int, error) {
	if len(args) != 1 {
		return 0, errors.New("msg takes exactly one message file")
	}
	content, err := os.ReadFile(args[0])
	if err != nil {
		return 0, fmt.Errorf("read message: %w", err)
	}
	found := 0
	for _, f := range Scan(KindCommit, string(content), Options{}) {
		fmt.Fprintf(stdout, "commit message:%s\n", f)
		found++
	}
	return found, nil
}

func runRange(ctx context.Context, args []string, stdout io.Writer) (int, error) {
	positional, title, bodyFile, err := parseRangeArgs(args)
	if err != nil {
		return 0, err
	}
	if len(positional) != 2 {
		return 0, errors.New("range takes <base> <head>")
	}
	base, head := positional[0], positional[1]

	found := 0
	commits, err := commitMessages(ctx, base, head)
	if err != nil {
		return 0, err
	}
	for _, c := range commits {
		for _, f := range Scan(KindCommit, c.message, Options{}) {
			fmt.Fprintf(stdout, "commit %s:%s\n", shortSHA(c.sha), f)
			found++
		}
	}

	allowBody := false
	if title != "" {
		allowBody = ProductScope(title)
		for _, f := range Scan(KindPR, title, Options{AllowClassB: allowBody}) {
			fmt.Fprintf(stdout, "pull request title:%s\n", f)
			found++
		}
	}
	if bodyFile != "" {
		body, err := os.ReadFile(bodyFile)
		if err != nil {
			return 0, fmt.Errorf("read pull request body: %w", err)
		}
		for _, f := range Scan(KindPR, string(body), Options{AllowClassB: allowBody}) {
			fmt.Fprintf(stdout, "pull request body:%s\n", f)
			found++
		}
	}
	return found, nil
}

// parseRangeArgs accepts positionals and flags in any order, because the CI
// call puts the flags after <base> <head>.
func parseRangeArgs(args []string) (positional []string, title, bodyFile string, err error) {
	flags := flag.NewFlagSet("range", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&title, "pr-title", "", "pull request title")
	flags.StringVar(&bodyFile, "pr-body-file", "", "file holding the pull request description")
	rest := args
	for len(rest) > 0 {
		if err = flags.Parse(rest); err != nil {
			return nil, "", "", fmt.Errorf("parse flags: %w", err)
		}
		rest = flags.Args()
		if len(rest) == 0 {
			break
		}
		positional = append(positional, rest[0])
		rest = rest[1:]
	}
	return positional, title, bodyFile, nil
}

func loadAllow(name string) ([]string, error) {
	f, err := os.Open(name) //nolint:gosec // path is a command-line flag of a developer tool
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	defer f.Close()
	globs, err := glob.ParseList(f, name)
	if err != nil {
		return nil, fmt.Errorf("parse allow list: %w", err)
	}
	return globs, nil
}

func trackedFiles(ctx context.Context) ([]string, error) {
	out, err := gitOutput(ctx, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			files = append(files, p)
		}
	}
	return files, nil
}

type commit struct {
	sha     string
	message string
}

// commitMessages returns the messages of the commits in base..head, oldest
// last (git log order). A base of all zeros (a push that created the branch)
// checks only the head commit.
func commitMessages(ctx context.Context, base, head string) ([]commit, error) {
	revs := []string{base + ".." + head}
	if base == zeroSHA {
		revs = []string{"-1", head}
	}
	args := append([]string{"log", "-z", "--format=%H%n%B"}, revs...)
	out, err := gitOutput(ctx, args...)
	if err != nil {
		return nil, err
	}
	var commits []commit
	for _, rec := range strings.Split(out, "\x00") {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		sha, msg, _ := strings.Cut(rec, "\n")
		commits = append(commits, commit{sha: sha, message: msg})
	}
	return commits, nil
}

func gitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errOut.String()))
	}
	return out.String(), nil
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
