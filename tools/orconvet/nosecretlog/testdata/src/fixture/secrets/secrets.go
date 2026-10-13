// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package secrets holds code that must be reported.
package secrets

import (
	"context"
	"fmt"
	"log/slog"
)

// Session has a tagged field with an innocent name.
type Session struct {
	ID    string
	Blob  string `orcon:"sensitive"`
	Other string `json:"other" orcon:"redacted,sensitive"`
}

// Config has secret-looking field names.
type Config struct {
	APIKey string
	Name   string
}

var globalToken = "x"

func identifiers(ctx context.Context, logger *slog.Logger, password, name string) {
	slog.Info("login", "password", password)           // want `sensitive value password passed to slog.Info`
	slog.InfoContext(ctx, "login", "user", password)   // want `sensitive value password passed to slog.InfoContext`
	logger.Warn("login", "who", name, "pw", password)  // want `sensitive value password passed to slog.Warn`
	slog.Info("attr", slog.String("secret", password)) // want `sensitive value password passed to slog.String`
	slog.Info("global", "t", globalToken)              // want `sensitive value globalToken passed to slog.Info`
}

func fields(logger *slog.Logger, s Session, c *Config) {
	logger.Info("session", "blob", s.Blob)   // want `sensitive value s.Blob passed to slog.Info`
	logger.Info("session", "other", s.Other) // want `sensitive value s.Other passed to slog.Info`
	logger.Error("config", "key", c.APIKey)  // want `sensitive value c.APIKey passed to slog.Error`
	logger.Info("config", "name", c.Name)    // clean: not a secret
	logger.Info("session", "id", s.ID)       // clean: not a secret
}

func errorsWrap(authorization string) error {
	return fmt.Errorf("call failed with %s", authorization) // want `sensitive value authorization passed to fmt.Errorf`
}

func nested(logger *slog.Logger, cookie string) {
	logger.Info("req", "h", fmt.Sprintf("%s", cookie)) // want `sensitive value cookie passed to slog.Info`
}
