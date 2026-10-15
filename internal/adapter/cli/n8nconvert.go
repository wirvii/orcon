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
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/n8nimport"
	"github.com/wirvii/orcon/internal/platform/id"
)

// n8nConvert runs "orcon n8n convert <workflow.json>". It writes the flow
// document and the import report. The exit code is 0 whenever the workflow
// was read: unsupported nodes are a result to report, not a failure.
func n8nConvert(_ context.Context, args []string, stdout, stderr io.Writer) int {
	var out, report, flowID string
	flags := flag.NewFlagSet("orcon n8n convert", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&out, "out", "", "write the flow document (JSON) to this file; stdout when empty")
	flags.StringVar(&report, "report", "", "write the import report (Markdown) to this file; stderr when empty")
	flags.StringVar(&flowID, "id", "", "id of the new flow (UUID); a new UUIDv7 when empty")
	// The flag package stops at the first positional argument, so the flags
	// are parsed again after each one: the file may come before or after them.
	var positional []string
	rest := args
	for {
		if err := flags.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return exitOK
			}
			return exitUsage
		}
		if flags.NArg() == 0 {
			break
		}
		positional = append(positional, flags.Arg(0))
		rest = flags.Args()[1:]
	}
	if len(positional) != 1 {
		_, _ = fmt.Fprintln(stderr, "usage: orcon n8n convert <workflow.json> [--out flow.json] [--report report.md] [--id uuid]")
		return exitUsage
	}
	if err := runConvert(positional[0], out, report, flowID, stdout, stderr); err != nil {
		_, _ = fmt.Fprintln(stderr, "orcon n8n convert:", err)
		return exitError
	}
	return exitOK
}

func runConvert(input, out, report, flowID string, stdout, stderr io.Writer) error {
	data, err := os.ReadFile(input) //nolint:gosec // G304: the operator names the workflow file to convert
	if err != nil {
		return fmt.Errorf("read workflow: %w", err)
	}
	id, err := newFlowID(flowID)
	if err != nil {
		return err
	}
	conv, err := n8nimport.ConvertWorkflow(data, id, n8nimport.Installed())
	if err != nil {
		return err
	}
	flowData, err := conv.FlowJSON()
	if err != nil {
		return err
	}
	if out == "" {
		if _, err := stdout.Write(append(flowData, '\n')); err != nil {
			return err
		}
	} else if err := writeFile(out, append(flowData, '\n')); err != nil {
		return err
	}
	md := []byte(conv.Markdown())
	if report == "" {
		_, err = stderr.Write(md)
		return err
	}
	return writeFile(report, md)
}

// newFlowID returns the id the caller gave, or a new UUIDv7.
func newFlowID(given string) (flow.ID, error) {
	if given != "" {
		if _, err := id.Parse(given); err != nil {
			return "", fmt.Errorf("--id: %w", err)
		}
		return flow.ID(given), nil
	}
	return flow.ID(id.NewV7Generator(time.Now).New().String()), nil
}
