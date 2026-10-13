// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package clean holds code that must not be reported.
package clean

import (
	"fmt"
	"log/slog"

	"fixture/internal/platform/redact"
)

type credentials struct {
	Token redact.Secret
	Label string
}

func redacted(logger *slog.Logger, token redact.Secret, c credentials) error {
	logger.Info("login", "token", token)
	logger.Info("login", "token", c.Token)
	logger.Info("login", "token", redact.Secret("raw"))
	return fmt.Errorf("rejected %v", token)
}

func harmless(logger *slog.Logger, user string, count int) {
	const tokenKey = "token" // a constant key is not a value
	logger.Info("login", tokenKey, count, "user", user)
	fmt.Println("not a logging call", user)
}

func shortNames(logger *slog.Logger, tk string) {
	logger.Info("login", "tk", tk)
}
