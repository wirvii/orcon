// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/wirvii/orcon/internal/platform/config"
)

// Exit codes of ADR 0058 section 4 that the commands of this package return.
const (
	// ExitOK means success.
	ExitOK = 0
	// ExitError is a generic failure.
	ExitError = 1
	// ExitUsage means the arguments are wrong.
	ExitUsage = 2
	// ExitPrecondition means the command cannot run in the current state.
	ExitPrecondition = 4
	// ExitValidation means the configuration (or the input) is invalid.
	ExitValidation = 5
)

// ErrProbeUnavailable is returned by a probe that cannot run in this build. The
// command reports it as skipped, not as a failure.
var ErrProbeUnavailable = errors.New("not available in this build")

// Probe is one connection check of orcon config check --probe: a database, a KMS,
// a backup target, an SMTP server. It reads and never writes.
type Probe struct {
	// Name is shown in the output, for example "database".
	Name string
	// Applies tells whether the configuration uses the thing the probe checks.
	Applies func(cfg *config.Config) bool
	// Run checks the connection. The context carries the 10 second deadline.
	Run func(ctx context.Context, cfg *config.Config) error
}

// Env is everything a command needs from the outside.
type Env struct {
	// Stdout and Stderr receive the output of the command. Results go to Stdout, diagnostics to Stderr.
	Stdout, Stderr io.Writer
	// Environ is the environment in os.Environ form.
	Environ []string
	// FS reads the configuration file and the secret files. Nil means the real file system.
	FS config.FileSystem
	// Decode parses the YAML configuration file (internal/adapter/codec/configyaml).
	Decode config.FileDecoder
	// MarshalYAML writes the YAML of orcon config check -o yaml.
	MarshalYAML func(v any) ([]byte, error)
	// GOOS names the operating system. Empty means the running one.
	GOOS string
	// IsTerminal tells whether Stdout is a terminal; it picks the default output format (ADR 0058).
	IsTerminal bool
	// Probes are the connection checks available to --probe.
	Probes []Probe
	// Reload asks the local instance to reread its configuration.
	Reload func(ctx context.Context, cfg *config.Config) error
	// Schema is the configuration schema. Nil means [config.Default].
	Schema *config.Schema
}

func (e Env) schema() *config.Schema {
	if e.Schema != nil {
		return e.Schema
	}
	return config.Default()
}

// Run executes the command named by args[0].
// The result handled is false when args do not name a command of this package,
// so the caller can treat them as server flags.
func Run(ctx context.Context, args []string, env Env) (code int, handled bool) {
	if len(args) == 0 {
		return 0, false
	}
	rest := args[1:]
	switch args[0] {
	case "config":
		return runConfig(ctx, rest, env), true
	case "docs":
		return runDocs(rest, env), true
	default:
		return 0, false
	}
}

func usagef(env Env, format string, a ...any) int {
	_, _ = fmt.Fprintf(env.Stderr, "orcon: "+format+"\n", a...)
	return ExitUsage
}
