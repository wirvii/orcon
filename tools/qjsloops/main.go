// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command qjsloops lists, in the (already patched) source of QuickJS-ng, every
// C function with a loop whose bound depends on its input or a memory move of
// variable length, and says whether the function polls or charges the
// interrupt counter (SPEC-001 amendment 10, section C). internal/adapter/wazero
// /assets/build.sh runs it and keeps the result in assets/native-loops.txt, so
// a change of the patch or of the QuickJS-ng version shows up in review.
//
// Usage:
//
//	qjsloops -src quickjs.c [-out native-loops.txt]
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "qjsloops:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("qjsloops", flag.ContinueOnError)
	src := flags.String("src", "", "path of quickjs.c with the Orcon patch applied")
	out := flags.String("out", "", "file to write (default standard output)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *src == "" {
		return fmt.Errorf("-src is required")
	}
	text, err := os.ReadFile(*src) //nolint:gosec // the path is the flag of a build script
	if err != nil {
		return err
	}
	report := Render(Scan(string(text)))
	if *out == "" {
		_, err = io.WriteString(stdout, report)
		return err
	}
	return os.WriteFile(*out, []byte(report), 0o644) //nolint:gosec // a committed text file
}
