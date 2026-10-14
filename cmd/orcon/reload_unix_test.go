// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"strings"
	"syscall"
	"testing"
)

// TestSIGHUPReloads checks that SIGHUP produces the same reload as the request file (R-CONF-09).
func TestSIGHUPReloads(t *testing.T) {
	s := startReloadServer(t, "log:\n  level: info\n")
	s.write(t, "log:\n  level: debug\n")
	// The server registered for SIGHUP before it reported that it listens, so the signal is not fatal here.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the reload log record", func() bool {
		return strings.Contains(s.stderr.String(), `"msg":"configuration reloaded"`) && strings.Contains(s.stderr.String(), `"applied":["log.level"]`)
	})
}
