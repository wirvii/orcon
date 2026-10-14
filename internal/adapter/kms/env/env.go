// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package env is the `env` master key provider (ADR-0042 section 2): the key
// is the base64 text of 32 bytes in ORCON_MASTER_KEY. It is an adapter
// package: it implements the envelope.MasterKey port of the credential area,
// and only cmd/orcon imports it.
package env

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wirvii/orcon/internal/credential/envelope"
)

// VarName is the environment variable the provider reads.
const VarName = "ORCON_MASTER_KEY"

// Provider is the env master key provider. It is the least protected provider,
// because the environment of a process is readable by tools and child
// processes, so starting it logs a warning.
type Provider struct {
	*envelope.LocalMasterKey
}

var _ envelope.MasterKey = (*Provider)(nil)

// New reads [VarName] through getenv, normally [os.Getenv], so tests do not
// touch the process environment. A nil logger discards the startup warning.
func New(ctx context.Context, getenv func(string) string, logger *slog.Logger) (*Provider, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	value := getenv(VarName)
	if value == "" {
		return nil, fmt.Errorf("%w: %s is not set", envelope.ErrMasterKeyInvalid, VarName)
	}
	key, err := envelope.ParseKeyMaterial([]byte(value), false)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", VarName, err)
	}
	logger.WarnContext(ctx, "master key read from the process environment; prefer the file provider or a key management service",
		slog.String("provider", "env"), slog.String("variable", VarName))
	return &Provider{envelope.NewLocalMasterKey("env", "env:"+VarName, key)}, nil
}
