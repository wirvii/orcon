// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/wirvii/orcon/internal/n8nimport"
)

// Exit codes of the commands.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

// Catalog names accepted by --catalog besides a file path.
const (
	catalogCore      = "core"
	catalogPlanned   = "planned"
	catalogInstalled = "installed"
)

// N8n runs "orcon n8n <command>". The args exclude the word "n8n".
func N8n(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		_, _ = fmt.Fprintln(stdout, "usage: orcon n8n coverage --corpus <dir> [--catalog core|planned|<file>] [flags]")
		if len(args) == 0 {
			return exitUsage
		}
		return exitOK
	}
	switch args[0] {
	case "coverage":
		return n8nCoverage(ctx, args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "orcon n8n: unknown command %q\n", args[0])
		return exitUsage
	}
}

type coverageFlags struct {
	corpus, catalog, out, markdown, roadmap, catalogOut, inventory string
	target                                                         float64
	maxConnectors                                                  int
}

func n8nCoverage(_ context.Context, args []string, stdout, stderr io.Writer) int {
	var f coverageFlags
	flags := flag.NewFlagSet("orcon n8n coverage", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&f.corpus, "corpus", "", "directory of the template corpus written by tools/n8ncorpus (required)")
	flags.StringVar(&f.catalog, "catalog", catalogPlanned,
		"catalog to measure: core (core nodes only), planned (core plus the connectors the wave-1 selection chooses), or the path of a catalog file")
	flags.Float64Var(&f.target, "target", n8nimport.DefaultTarget, "percentage of templates the selection aims for")
	flags.IntVar(&f.maxConnectors, "max-connectors", 0, "stop the selection after this many connectors (0 means no limit)")
	flags.StringVar(&f.out, "out", "", "write the JSON report to this file")
	flags.StringVar(&f.markdown, "markdown", "", "write the coverage page (Markdown) to this file")
	flags.StringVar(&f.roadmap, "roadmap", "", "write the wave-1 selection page (Markdown) to this file")
	flags.StringVar(&f.catalogOut, "catalog-out", "", "write the planned catalog (JSON) to this file")
	flags.StringVar(&f.inventory, "inventory", "", "inventory of the pinned n8n release (JSON); fails when a core type has no counterpart")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if f.corpus == "" {
		_, _ = fmt.Fprintln(stderr, "orcon n8n coverage: --corpus is required")
		return exitUsage
	}
	if f.target <= 0 || f.target > 100 {
		_, _ = fmt.Fprintln(stderr, "orcon n8n coverage: --target must be between 0 and 100")
		return exitUsage
	}
	if err := runCoverage(&f, stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, "orcon n8n coverage:", err)
		return exitError
	}
	return exitOK
}

func runCoverage(f *coverageFlags, stdout io.Writer) error {
	corpus, err := n8nimport.LoadCorpus(os.DirFS(f.corpus))
	if err != nil {
		return err
	}
	var (
		cat   *n8nimport.Catalog
		sel   *n8nimport.Selection
		label = f.catalog
	)
	switch f.catalog {
	case catalogCore:
	case catalogInstalled:
		return errors.New("--catalog installed is not available yet: the installed connector store arrives with the connector manifest cut (CN1)")
	case catalogPlanned:
		sel, err = n8nimport.SelectWave(corpus, nil, n8nimport.SelectOptions{Target: f.target, MaxConnectors: f.maxConnectors})
		if err != nil {
			return err
		}
		cat = sel.Catalog
	default:
		data, rerr := os.ReadFile(f.catalog)
		if rerr != nil {
			return fmt.Errorf("read catalog: %w", rerr)
		}
		cat, err = n8nimport.ParseCatalog(data)
		if err != nil {
			return err
		}
		label = "file"
	}
	report := n8nimport.Measure(corpus, cat, label)

	if f.out != "" {
		data, err := report.JSON(sel)
		if err != nil {
			return err
		}
		if err := writeFile(f.out, append(data, '\n')); err != nil {
			return err
		}
	}
	if f.markdown != "" {
		if err := writeFile(f.markdown, []byte(report.Markdown())); err != nil {
			return err
		}
	}
	if sel != nil && f.roadmap != "" {
		if err := writeFile(f.roadmap, []byte(sel.RoadmapMarkdown(report, corpus.Manifest))); err != nil {
			return err
		}
	}
	if sel != nil && f.catalogOut != "" {
		data, err := sel.Catalog.Encode()
		if err != nil {
			return err
		}
		if err := writeFile(f.catalogOut, append(data, '\n')); err != nil {
			return err
		}
	}
	printSummary(stdout, report, sel)
	if f.inventory != "" {
		return checkInventory(f.inventory)
	}
	return nil
}

func checkInventory(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // G304: the inventory path is a command-line argument
	if err != nil {
		return fmt.Errorf("read inventory: %w", err)
	}
	inv, err := n8nimport.ParseInventory(data)
	if err != nil {
		return err
	}
	if gaps := n8nimport.CheckInventory(inv); len(gaps) > 0 {
		return fmt.Errorf("%d core node types of n8n %s have no counterpart: %v", len(gaps), inv.N8nVersion, gaps)
	}
	return nil
}

func writeFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o600); err != nil { //nolint:gosec // G703: the path is a command-line argument
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func printSummary(w io.Writer, r *n8nimport.Report, sel *n8nimport.Selection) {
	c := r.Counts
	_, _ = fmt.Fprintf(w, "corpus: %d templates (%d unreadable), n8n %s\n", c.Templates, r.Unreadable, r.Manifest.N8nVersion)
	_, _ = fmt.Fprintf(w, "catalog: %s\n", r.Catalog)
	_, _ = fmt.Fprintf(w, "fully importable: %.2f %% today, %.2f %% with the AI nodes mapped (unweighted)\n",
		n8nimport.Pct(int64(c.Importable), int64(c.Templates)), n8nimport.Pct(int64(c.ImportableIgnoringAI), int64(c.Templates)))
	_, _ = fmt.Fprintf(w, "weighted by views: %.2f %% today, %.2f %% with the AI nodes mapped\n",
		n8nimport.Pct(c.ViewsImportable, c.Views), n8nimport.Pct(c.ViewsImportableIgnoringAI, c.Views))
	if sel != nil {
		_, _ = fmt.Fprintf(w, "wave 1: %d connectors, stop: %s, ceiling %.2f %%\n", len(sel.Steps), sel.Stop,
			n8nimport.Pct(int64(sel.Ceiling), int64(sel.Templates)))
	}
}
