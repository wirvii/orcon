// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/wirvii/orcon/internal/platform/config"
)

// probeTimeout is the deadline of each connection check of --probe (R-CONF-07).
const probeTimeout = 10 * time.Second

const configUsage = `usage: orcon config <command> [flags]

commands:
  check     print every effective setting, where it came from, and every problem
  schema    print the JSON Schema of the configuration file
  reload    ask the local instance to reread its configuration file

Every setting is also a flag (see orcon config check --help); --config names the YAML file.
`

func runConfig(ctx context.Context, args []string, env Env) int {
	if len(args) == 0 {
		_, _ = io.WriteString(env.Stderr, configUsage)
		return ExitUsage
	}
	switch args[0] {
	case "check":
		return configCheck(ctx, args[1:], env)
	case "schema":
		return configSchema(args[1:], env)
	case "reload":
		return configReload(ctx, args[1:], env)
	case "help", "-h", "--help":
		_, _ = io.WriteString(env.Stdout, configUsage)
		return ExitOK
	default:
		_, _ = fmt.Fprintf(env.Stderr, "orcon: unknown config command %q\n\n%s", args[0], configUsage)
		return ExitUsage
	}
}

// parsed is a command line prepared against the schema.
type parsed struct {
	flags  *flag.FlagSet
	values *config.FlagValues
}

func newFlagSet(name string, env Env) parsed {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return parsed{flags: fs, values: env.schema().BindFlags(fs)}
}

// parse runs the flags. A help request prints the usage and reports done; a bad flag reports the usage exit code.
func (p parsed) parse(args []string, env Env) (code int, done bool) {
	err := env.schema().ParseFlags(p.flags, args)
	switch {
	case err == nil:
		return ExitOK, false
	case errors.Is(err, flag.ErrHelp):
		printFlags(env.Stdout, p.flags)
		return ExitOK, true
	default:
		_, _ = fmt.Fprintf(env.Stderr, "orcon: %v\n", err)
		return ExitUsage, true
	}
}

func printFlags(w io.Writer, fs *flag.FlagSet) {
	_, _ = fmt.Fprintf(w, "usage: %s [flags]\n\nflags:\n", fs.Name())
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
}

func (e Env) load(p parsed) *config.Result {
	return e.schema().Load(config.Input{
		Flags:   p.values,
		Environ: e.Environ,
		FS:      e.FS,
		Decode:  e.Decode,
		GOOS:    e.GOOS,
	})
}

func configCheck(ctx context.Context, args []string, env Env) int {
	p := newFlagSet("orcon config check", env)
	output := p.flags.String("o", "", "output format: table, json or yaml (default: table on a terminal, json otherwise)")
	probe := p.flags.Bool("probe", false, "also check the connections: database, key management service, backup target, SMTP (10 s each, nothing is written)")
	if code, done := p.parse(args, env); done {
		return code
	}
	format := *output
	if format == "" {
		format = "json"
		if env.IsTerminal {
			format = "table"
		}
	}
	if !slices.Contains([]string{"table", "json", "yaml"}, format) {
		return usagef(env, "-o must be table, json or yaml, not %q", format)
	}

	res := env.load(p)
	var probes []probeResult
	if *probe {
		probes = runProbes(ctx, env, res)
	}
	rep := buildReport(res, probes)
	if err := writeReport(env, format, rep); err != nil {
		_, _ = fmt.Fprintf(env.Stderr, "orcon: %v\n", err)
		return ExitError
	}
	if !rep.Valid {
		return ExitValidation
	}
	return ExitOK
}

// report is the shape printed by orcon config check, in every format.
type report struct {
	Valid      bool          `json:"valid" yaml:"valid"`
	ConfigFile string        `json:"configFile" yaml:"configFile"`
	Entries    []reportEntry `json:"entries" yaml:"entries"`
	Issues     []reportIssue `json:"issues" yaml:"issues"`
	Probes     []probeResult `json:"probes,omitempty" yaml:"probes,omitempty"`
}

type reportEntry struct {
	Key    string `json:"key" yaml:"key"`
	Value  any    `json:"value" yaml:"value"`
	Source string `json:"source" yaml:"source"`
	Secret bool   `json:"secret" yaml:"secret"`
	Reload string `json:"reload" yaml:"reload"`
}

type reportIssue struct {
	Severity string `json:"severity" yaml:"severity"`
	Key      string `json:"key" yaml:"key"`
	Message  string `json:"message" yaml:"message"`
}

type probeResult struct {
	Name   string `json:"name" yaml:"name"`
	Status string `json:"status" yaml:"status"`
	Detail string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

func buildReport(res *config.Result, probes []probeResult) report {
	rep := report{Valid: !res.HasErrors(), ConfigFile: res.Config.File(), Probes: probes}
	entries := res.Config.Entries()
	for i := range entries {
		e := &entries[i]
		rep.Entries = append(rep.Entries, reportEntry{
			Key:    e.Key.Path,
			Value:  jsonValue(e),
			Source: e.Source.String(),
			Secret: e.Key.Secret,
			Reload: e.Key.Reload.String(),
		})
	}
	rep.Issues = []reportIssue{}
	for _, i := range res.Issues {
		rep.Issues = append(rep.Issues, reportIssue{Severity: strings.ToLower(i.Severity.String()), Key: i.Key, Message: i.Message})
	}
	for _, pr := range probes {
		if pr.Status == "failed" {
			rep.Valid = false
		}
	}
	return rep
}

// jsonValue returns the value typed for JSON and YAML. A secret is never returned: it is [REDACTED] or empty.
func jsonValue(e *config.Entry) any {
	v := e.Value
	if e.Key.Secret {
		return v.String()
	}
	switch e.Key.Kind {
	case config.KindInt:
		if !v.IsZero() {
			return v.Int()
		}
	case config.KindFloat:
		if !v.IsZero() {
			return v.Float()
		}
	case config.KindBool:
		if !v.IsZero() {
			return v.Bool()
		}
	case config.KindList:
		return v.List()
	case config.KindMap:
		return v.Map()
	case config.KindString, config.KindDuration, config.KindEnum, config.KindPath, config.KindURL, config.KindAddress, config.KindBytes:
	}
	return v.Text()
}

func writeReport(env Env, format string, rep report) error {
	switch format {
	case "json":
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(rep); err != nil {
			return fmt.Errorf("encode JSON: %w", err)
		}
		_, err := env.Stdout.Write(buf.Bytes())
		return wrapWrite(err)
	case "yaml":
		if env.MarshalYAML == nil {
			return errors.New("this build cannot write YAML")
		}
		out, err := env.MarshalYAML(rep)
		if err != nil {
			return err
		}
		_, err = env.Stdout.Write(out)
		return wrapWrite(err)
	default:
		return writeTable(env.Stdout, rep)
	}
}

func wrapWrite(err error) error {
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func tableValue(v any) string {
	switch x := v.(type) {
	case []string:
		if len(x) == 0 {
			return "[]"
		}
		return strings.Join(x, ",")
	case map[string]string:
		if len(x) == 0 {
			return "{}"
		}
		names := make([]string, 0, len(x))
		for n, val := range x {
			names = append(names, n+"="+val)
		}
		slices.Sort(names)
		return strings.Join(names, ",")
	case string:
		if x == "" {
			return `""`
		}
		return x
	default:
		return fmt.Sprint(x)
	}
}

func writeTable(w io.Writer, rep report) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE\tRELOAD")
	for _, e := range rep.Entries {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Key, tableValue(e.Value), e.Source, e.Reload)
	}
	if err := tw.Flush(); err != nil {
		return wrapWrite(err)
	}
	if len(rep.Issues) > 0 || len(rep.Probes) > 0 {
		_, _ = fmt.Fprintln(w)
	}
	for _, i := range rep.Issues {
		_, _ = fmt.Fprintf(w, "%-5s  %s: %s\n", strings.ToUpper(i.Severity), i.Key, i.Message)
	}
	for _, p := range rep.Probes {
		line := fmt.Sprintf("PROBE  %s: %s", p.Name, p.Status)
		if p.Detail != "" {
			line += " - " + p.Detail
		}
		_, _ = fmt.Fprintln(w, line)
	}
	if rep.Valid && len(rep.Issues) == 0 {
		_, _ = fmt.Fprintln(w, "\nThe configuration is valid.")
	}
	return nil
}

// runProbes runs the connection checks that apply, each with its own deadline.
// A configuration with errors is not probed: its values cannot be trusted.
func runProbes(ctx context.Context, env Env, res *config.Result) []probeResult {
	var out []probeResult
	if res.HasErrors() {
		return []probeResult{{Name: "all", Status: "skipped", Detail: "the configuration has errors"}}
	}
	for _, p := range env.Probes {
		if p.Applies != nil && !p.Applies(res.Config) {
			out = append(out, probeResult{Name: p.Name, Status: "skipped", Detail: "not configured"})
			continue
		}
		pctx, cancel := context.WithTimeout(ctx, probeTimeout)
		err := p.Run(pctx, res.Config)
		cancel()
		if errors.Is(err, ErrProbeUnavailable) {
			out = append(out, probeResult{Name: p.Name, Status: "skipped", Detail: ErrProbeUnavailable.Error()})
			continue
		}
		if err != nil {
			out = append(out, probeResult{Name: p.Name, Status: "failed", Detail: scrub(res.Config, err.Error())})
			continue
		}
		out = append(out, probeResult{Name: p.Name, Status: "ok"})
	}
	return out
}

// scrub removes every secret value from a message that came from a driver.
func scrub(cfg *config.Config, msg string) string {
	entries := cfg.Entries()
	for i := range entries {
		e := entries[i]
		if !e.Key.Secret || e.Value.IsZero() {
			continue
		}
		for _, secret := range secretParts(&e) {
			if len(secret) >= 3 {
				msg = strings.ReplaceAll(msg, secret, config.Redacted)
			}
		}
	}
	return msg
}

func secretParts(e *config.Entry) []string {
	if e.Key.Kind == config.KindMap {
		var parts []string
		for _, v := range e.Value.Map() {
			parts = append(parts, v)
		}
		return parts
	}
	return []string{e.Value.Secret().Reveal()}
}

func configSchema(args []string, env Env) int {
	fs := flag.NewFlagSet("orcon config schema", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(env.Stdout, "usage: orcon config schema\n\nPrints the JSON Schema of the YAML configuration file.\n")
			return ExitOK
		}
		return usagef(env, "%v", err)
	}
	if fs.NArg() > 0 {
		return usagef(env, "config schema takes no arguments")
	}
	out, err := JSONSchema(env.schema())
	if err != nil {
		_, _ = fmt.Fprintf(env.Stderr, "orcon: %v\n", err)
		return ExitError
	}
	if _, err := env.Stdout.Write(out); err != nil {
		_, _ = fmt.Fprintf(env.Stderr, "orcon: write output: %v\n", err)
		return ExitError
	}
	return ExitOK
}

func configReload(ctx context.Context, args []string, env Env) int {
	p := newFlagSet("orcon config reload", env)
	if code, done := p.parse(args, env); done {
		return code
	}
	res := env.load(p)
	if err := res.Err(); err != nil {
		_, _ = fmt.Fprintf(env.Stderr, "orcon: the configuration has errors, so the instance was not asked to reload it\n%v\n", err)
		return ExitValidation
	}
	if env.Reload == nil {
		_, _ = fmt.Fprintln(env.Stderr, "orcon: this build cannot reach a local instance")
		return ExitPrecondition
	}
	if err := env.Reload(ctx, res.Config); err != nil {
		_, _ = fmt.Fprintf(env.Stderr, "orcon: %v\n", err)
		return ExitPrecondition
	}
	_, _ = fmt.Fprintln(env.Stdout, "The local instance was asked to reread its configuration. Settings that need a restart keep their value; see orcon config check.")
	return ExitOK
}
