// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command spdxcheck verifies (and with -fix writes) the SPDX header of every
// file it is given. The holder must be exactly "The Orcon Authors" and the
// license "Apache-2.0".
//
// Usage:
//
//	git ls-files | spdxcheck [-fix] [-exempt .spdx-exempt]
//	spdxcheck [-fix] [-exempt .spdx-exempt] path...
//
// The exempt file holds one "glob  # reason" per line; the reason is required.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wirvii/orcon/tools/internal/glob"
	"github.com/wirvii/orcon/tools/provenance/configpaths"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) //nolint:forbidigo // tool entry point owns the process exit code
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("spdxcheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fix := flags.Bool("fix", false, "write missing or wrong headers instead of only reporting them")
	exemptFile := flags.String("exempt", ".spdx-exempt", "file listing exempt globs, each with a reason")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	exempt, err := loadExempt(*exemptFile)
	if err != nil {
		fmt.Fprintln(stderr, "spdxcheck:", err)
		return 2
	}
	// Tool-managed configuration is rewritten by tooling and never carries headers.
	exempt = append(exempt, configpaths.Tooling...)

	paths := flags.Args()
	if len(paths) == 0 {
		paths, err = readPaths(stdin)
		if err != nil {
			fmt.Fprintln(stderr, "spdxcheck: read file list:", err)
			return 2
		}
	}

	failed := 0
	for _, p := range paths {
		if glob.MatchAny(exempt, p) {
			continue
		}
		content, err := os.ReadFile(p) //nolint:gosec // paths come from git ls-files or the command line
		if errors.Is(err, os.ErrNotExist) {
			continue // listed but deleted in the working tree
		}
		if err != nil {
			fmt.Fprintln(stderr, "spdxcheck:", err)
			return 2
		}
		res := Check(p, content)
		if len(res.Findings) == 0 {
			continue
		}
		if *fix && res.Fixed != nil {
			if err := os.WriteFile(p, res.Fixed, 0o644); err != nil { //nolint:gosec // preserves a normal repository file mode
				fmt.Fprintln(stderr, "spdxcheck:", err)
				return 2
			}
			fmt.Fprintln(stdout, "fixed", p)
			continue
		}
		for _, f := range res.Findings {
			fmt.Fprintln(stdout, f)
		}
		failed++
	}
	if failed > 0 {
		fmt.Fprintf(stderr, "spdxcheck: %d file(s) without a valid SPDX header; run `make spdx-fix`\n", failed)
		return 1
	}
	return 0
}

func readPaths(r io.Reader) ([]string, error) {
	var paths []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			paths = append(paths, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	return paths, nil
}

// loadExempt reads the exempt globs. A missing file means no exemptions.
func loadExempt(name string) ([]string, error) {
	f, err := os.Open(name) //nolint:gosec // path is a command-line flag of a developer tool
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	defer f.Close()
	return glob.ParseList(f, name)
}
