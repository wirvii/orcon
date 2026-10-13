// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command benchgate is the performance regression gate (SPEC-008 CFG-7). It
// reads benchstat's CSV comparison of the base against the pull request head
// and fails when sec/op, B/op or allocs/op got worse by more than 10 % with
// p < 0.05, unless the pull request carries the accepted label and a written
// justification.
//
// Usage:
//
//	benchstat -format csv base.txt head.txt | benchgate [-label-accepted] [-pr-body-file body.md]
//	benchgate -csv comparison.csv [-label-accepted] [-pr-body-file body.md]
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) //nolint:forbidigo // tool entry point owns the process exit code
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("benchgate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	csvFile := flags.String("csv", "", "benchstat csv file (default: standard input)")
	accepted := flags.Bool("label-accepted", false, "the pull request carries the performance-regression-accepted label")
	bodyFile := flags.String("pr-body-file", "", "file holding the pull request description")
	threshold := flags.Float64("threshold", defaultThreshold, "regression threshold in percent")
	alpha := flags.Float64("alpha", defaultAlpha, "significance level")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	in := stdin
	if *csvFile != "" {
		f, err := os.Open(*csvFile)
		if err != nil {
			fmt.Fprintln(stderr, "benchgate:", err)
			return 2
		}
		defer f.Close()
		in = f
	}
	rows, err := ParseCSV(in)
	if err != nil {
		fmt.Fprintln(stderr, "benchgate:", err)
		return 2
	}
	if len(rows) == 0 {
		fmt.Fprintln(stderr, "benchgate:", errNoRows)
		return 2
	}

	var body string
	if *bodyFile != "" {
		b, err := os.ReadFile(*bodyFile)
		if err != nil {
			fmt.Fprintln(stderr, "benchgate:", err)
			return 2
		}
		body = string(b)
	}

	res := Gate(rows, Options{Threshold: *threshold, Alpha: *alpha, Accepted: *accepted, Body: body})
	for _, r := range res.Regressions {
		fmt.Fprintln(stdout, "regression:", r)
	}
	if res.Pass {
		fmt.Fprintln(stdout, "benchgate: pass:", res.Message)
		return 0
	}
	fmt.Fprintln(stderr, "benchgate: fail:", res.Message)
	return 1
}
