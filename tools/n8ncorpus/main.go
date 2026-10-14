// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command n8ncorpus downloads the public n8n template library and extracts
// the expressions of its workflows, for the expression compatibility
// measurement (ADR-0018 section 6, SPEC-001 criterion E-EXPR-3).
//
// The templates are community-submitted content whose licence is not stated
// per template. They are therefore only ever written into a directory outside
// version control (default .cache/expr-corpus); what the repository commits is
// the manifest data: counts, the corpus digest and scores.
//
// Usage:
//
//	n8ncorpus -n8n 2.43.4 [-dir .cache/expr-corpus] [-limit 600] [-base https://api.n8n.io]
//
// It writes dir/templates/<id>.json (downloaded once, then reused),
// dir/expressions.jsonl (one expression per line) and dir/manifest.json.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code) //nolint:forbidigo // tool entry point owns the process exit code
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("n8ncorpus", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", ".cache/expr-corpus", "directory for the downloaded corpus (keep it out of version control)")
	version := flags.String("n8n", "", "pinned n8n release the measurement is made against (required)")
	limit := flags.Int("limit", 600, "number of most viewed templates to use")
	base := flags.String("base", defaultBase, "base URL of the template library")
	delay := flags.Duration("delay", 150*time.Millisecond, "pause between requests of one worker")
	workers := flags.Int("workers", 4, "parallel downloads")
	n8nURL := flags.String("n8n-url", "", "address of a locally running instance of the pinned n8n release; its node type list becomes inventory.json (without it no inventory is written and the coverage command refuses the corpus)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *version == "" {
		fmt.Fprintln(stderr, "n8ncorpus: -n8n is required: the measurement must name the pinned n8n release")
		return 2
	}
	if *limit < 1 || *workers < 1 {
		fmt.Fprintln(stderr, "n8ncorpus: -limit and -workers must be at least 1")
		return 2
	}
	f := &fetcher{
		base:    *base,
		client:  &http.Client{Timeout: 60 * time.Second},
		delay:   *delay,
		workers: *workers,
		userAg:  "orcon-expr-compat/1 (+https://github.com/wirvii/orcon)",
	}
	ids, err := f.topTemplates(ctx, *limit)
	if err != nil {
		fmt.Fprintln(stderr, "n8ncorpus:", err)
		return 1
	}
	if len(ids) == 0 {
		fmt.Fprintln(stderr, "n8ncorpus:", errNoTemplates)
		return 1
	}
	if err := f.fetchAll(ctx, *dir, ids); err != nil {
		fmt.Fprintln(stderr, "n8ncorpus:", err)
		return 1
	}
	m, err := extract(*dir, *version, *base+"/api/templates", ids)
	if err != nil {
		fmt.Fprintln(stderr, "n8ncorpus:", err)
		return 1
	}
	fmt.Fprintf(stdout, "n8ncorpus: %d templates, %d expressions, corpus %s\n", m.Templates, m.Expressions, m.CorpusSHA256)
	if *n8nURL == "" {
		fmt.Fprintln(stdout, "n8ncorpus: no inventory written: start the pinned n8n release locally and pass -n8n-url")
		return 0
	}
	inv, err := f.fetchInventory(ctx, *n8nURL, *version)
	if err != nil {
		fmt.Fprintln(stderr, "n8ncorpus:", err)
		return 1
	}
	digest, err := writeInventory(*dir, inv)
	if err != nil {
		fmt.Fprintln(stderr, "n8ncorpus:", err)
		return 1
	}
	m.InventorySHA256 = digest
	if err := updateManifest(*dir, m); err != nil {
		fmt.Fprintln(stderr, "n8ncorpus:", err)
		return 1
	}
	core, ai := 0, 0
	for _, n := range inv.Nodes {
		switch n.Group {
		case "CORE":
			core++
		case "AI":
			ai++
		}
	}
	fmt.Fprintf(stdout, "n8ncorpus: inventory of n8n %s: %d node types, %d core, %d AI, digest %s\n", inv.N8nVersion, len(inv.Nodes), core, ai, digest)
	return 0
}
