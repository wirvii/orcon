// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest_test

import (
	"os"
	"path/filepath"
	"testing"
)

// Environment of the real run (mk/api.mk, target api-examples).
const (
	envBinary = "ORCON_BIN"
	envDB     = "ORCON_EXAMPLES_DB"
	// The bundled contract, relative to this package.
	contractPath = "../../../../packages/openapi/v1/openapi.bundle.json"
)

// TestExamples runs the examples of the real contract against the real binary.
//
// It runs only the part that exists today: until the contract has a
// non-interactive credential procedure (SPEC-005 API3) there is nothing a
// server could be asked with, so the test says so and skips, and
// TestRunnerAgainstFake is the only proof of the runner. API3 replaces the skip
// with Launch + RunExamples.
func TestExamples(t *testing.T) {
	db := os.Getenv(envDB)
	switch db {
	case "", "sqlite", "postgres":
	default:
		t.Fatalf("%s=%q, want sqlite or postgres", envDB, db)
	}
	contract := filepath.Clean(contractPath)
	if _, err := os.Stat(contract); err != nil {
		t.Skipf("no contract at %s yet: only the self-test runs", contract)
	}
	if _, err := os.Stat(binaryPath()); err != nil {
		t.Skipf("no binary at %s (build it with make build-go): only the self-test runs", binaryPath())
	}
	t.Skip("no non-interactive credential procedure yet; this test runs the real examples once SPEC-005 API3 (the first non-interactive credential, tracked as BL-062) provides one, until then only TestRunnerAgainstFake proves the runner")
}

func binaryPath() string {
	if bin := os.Getenv(envBinary); bin != "" {
		return bin
	}
	return filepath.Clean("../../../../bin/orcon")
}
