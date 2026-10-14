// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command brandcheck enforces the brand asset rules of SPEC-018 section 3.4.
//
// Usage:
//
//	brandcheck [-root dir] all
//	brandcheck svg <file>...
//
// "all" checks the tree below the root (default: the current directory):
//
//	BC-1  every brand SVG is a true vector: no image, feImage, foreignObject,
//	      script, text, style, data: URI, base64 payload, external reference or
//	      url() that is not a local #id; a viewBox, a drawn shape, at most 16 KiB
//	BC-2  each copy declared in brand/manifest.json is byte-identical to its asset
//	BC-3  PNG sizes and ICO entries match the manifest
//	BC-4  mascot cut-outs: opaque pixels equal the board crop, transparent corners
//	BC-5  every asset file is listed, and an asset wired to the app is approved
//	BC-6  the reference boards keep the SHA-256 recorded in the README and manifest
//
// Without brand/manifest.json only BC-1 and BC-6 run and the command succeeds
// when they do. "svg" applies BC-1 alone to the named files.
//
// Findings print as "file:line: rule: detail" (line 1 for a whole file). The
// exit status is 0 when nothing is found, 1 when something is found and 2 for a
// usage or input error.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) //nolint:forbidigo // tool entry point owns the process exit code
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("brandcheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: brandcheck [-root dir] all | svg <file>...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	switch flags.Arg(0) {
	case "all":
		if flags.NArg() != 1 {
			flags.Usage()
			return 2
		}
		findings, notes, err := checkAll(*root)
		if err != nil {
			fmt.Fprintln(stderr, "brandcheck:", err)
			return 2
		}
		return report(stdout, stderr, findings, notes)
	case "svg":
		if flags.NArg() < 2 {
			flags.Usage()
			return 2
		}
		var findings []Finding
		for _, file := range flags.Args()[1:] {
			data, err := os.ReadFile(file)
			if err != nil {
				fmt.Fprintln(stderr, "brandcheck:", err)
				return 2
			}
			findings = append(findings, checkSVG(file, data)...)
		}
		sortFindings(findings)
		return report(stdout, stderr, findings, nil)
	default:
		flags.Usage()
		return 2
	}
}

func report(stdout, stderr io.Writer, findings []Finding, notes []string) int {
	for _, n := range notes {
		fmt.Fprintln(stdout, "note:", n)
	}
	for _, f := range findings {
		fmt.Fprintln(stdout, f)
	}
	if len(findings) > 0 {
		fmt.Fprintf(stderr, "brandcheck: %d finding(s)\n", len(findings))
		return 1
	}
	fmt.Fprintln(stdout, "brandcheck: ok")
	return 0
}
