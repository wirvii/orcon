// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/identity"
)

// TestAccountsSetNameUsage (BL-132): set-name needs an address and a name, takes no password and
// refuses stray arguments; none of these opens the store.
func TestAccountsSetNameUsage(t *testing.T) {
	cases := map[string][]string{
		"no address":     {"set-name", "--name", "Ana"},
		"no name":        {"set-name", "--email", "ana@example.com"},
		"blank name":     {"set-name", "--email", "ana@example.com", "--name", "   "},
		"stray argument": {"set-name", "--email", "ana@example.com", "--name", "Ana", "extra"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := runAccounts(t.Context(), args, strings.NewReader("never read\n"), &stdout, &stderr); code != exitUsage {
				t.Errorf("accounts %v = %d, want the usage exit code; stderr %q", args, code, stderr.String())
			}
			if stderr.Len() == 0 {
				t.Errorf("accounts %v printed no reason", args)
			}
		})
	}
}

// TestAccountsSetNameRenamesTheAccount (BL-132): the command changes the display name of an
// existing account on the SQLite store, reads nothing from standard input, writes the
// account.renamed entry and says nothing about a password; an unknown address fails.
func TestAccountsSetNameRenamesTheAccount(t *testing.T) {
	t.Setenv("ORCON_DATA_DIR", t.TempDir())
	t.Setenv("ORCON_STORE_DRIVER", "sqlite")
	var stdout, stderr bytes.Buffer

	if code := runAccounts(t.Context(), []string{"create", "--email", "owner@example.com", "--name", "Owner"}, strings.NewReader("correct horse battery staple\n"), &stdout, &stderr); code != exitOK {
		t.Fatalf("create = %d; stderr %q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runAccounts(t.Context(), []string{"set-name", "--email", "Owner@Example.com", "--name", "Juan Tamayo"}, strings.NewReader(""), &stdout, &stderr); code != exitOK {
		t.Fatalf("set-name = %d; stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "display name changed") || strings.Contains(stderr.String(), "password") {
		t.Errorf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}

	store, err := openAccountsStore(t.Context())
	if err != nil {
		t.Fatalf("open the store: %v", err)
	}
	user, err := store.GetUserByEmail(t.Context(), "owner@example.com")
	// SQLite is held by one process at a time, so the store is released before the next command.
	if err := store.Close(); err != nil {
		t.Fatalf("close the store: %v", err)
	}
	if err != nil || user.DisplayName != "Juan Tamayo" {
		t.Fatalf("user after set-name = %+v, %v; want the new name", user, err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runAccounts(t.Context(), []string{"set-name", "--email", "nobody@example.com", "--name", "Nobody"}, strings.NewReader(""), &stdout, &stderr); code != exitError {
		t.Errorf("unknown address = %d, want the error exit code; stderr %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), identity.ErrNotFound.Error()) {
		t.Errorf("stderr %q does not say the account was not found", stderr.String())
	}
}
