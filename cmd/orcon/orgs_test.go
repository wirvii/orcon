// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestOrgsCreateRequiresItsFlags: a missing slug, name or administrator is a usage error, and the
// command does not open the store.
func TestOrgsCreateRequiresItsFlags(t *testing.T) {
	cases := [][]string{
		{"create"},
		{"create", "--slug", "acme", "--name", "Acme"},
		{"create", "--slug", "acme", "--admin", "ana@example.com"},
		{"create", "--name", "Acme", "--admin", "ana@example.com"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		if code := runOrgs(t.Context(), args, &stdout, &stderr); code != exitUsage {
			t.Errorf("orgs %v = %d, want the usage exit code; stderr %q", args, code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "is required") && stderr.Len() == 0 && stdout.Len() == 0 {
			t.Errorf("orgs %v printed nothing", args)
		}
	}
}

// TestOrgsRejectsAnUnknownSubcommand: only create exists.
func TestOrgsRejectsAnUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runOrgs(t.Context(), []string{"delete"}, &stdout, &stderr); code != exitUsage {
		t.Errorf("orgs delete = %d, want the usage exit code", code)
	}
	if !strings.Contains(stderr.String(), `unknown orgs subcommand "delete"`) {
		t.Errorf("stderr = %q", stderr.String())
	}
}
