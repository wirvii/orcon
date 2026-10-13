// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

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
// a backup target, an SMTP server. It reads and never writes. Whoever registers the
// key of an outgoing connection registers its probe.
type Probe struct {
	// Name is shown in the output, for example "database".
	Name string
	// Key is the configuration key the probe checks, shown with the result.
	Key string
	// Applies tells whether the configuration uses the thing the probe checks.
	Applies func(cfg *config.Config) bool
	// Run checks the connection. The context carries the 10 second deadline. The
	// detail is shown next to a successful result, for example a warning.
	Run func(ctx context.Context, cfg *config.Config) (detail string, err error)
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
	// ReloadWait is how long orcon config reload waits for a server. Zero means [ReloadWait].
	ReloadWait time.Duration
	// ReloadPoll is how often it looks for the answer. Zero means 100 ms.
	ReloadPoll time.Duration
	// Schema is the configuration schema. Nil means [config.Default].
	Schema *config.Schema
}

func (e Env) schema() *config.Schema {
	if e.Schema != nil {
		return e.Schema
	}
	return config.Default()
}

// RunConfig runs orcon config <command>; args are the arguments after "config".
func RunConfig(ctx context.Context, args []string, env Env) int { return runConfig(ctx, args, env) }

// RunDocs runs orcon docs <command>; args are the arguments after "docs".
func RunDocs(args []string, env Env) int { return runDocs(args, env) }

func usagef(env Env, format string, a ...any) int {
	_, _ = fmt.Fprintf(env.Stderr, "orcon: "+format+"\n", a...)
	return ExitUsage
}
