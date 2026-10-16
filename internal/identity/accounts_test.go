// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"errors"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/platform/id"
)

// TestCreateAccountStoresNormalizedAddressAndHash (SPEC-004 R-AUTH-01): the address is
// normalized, the password is stored only as its hash, and the creation is audited in the
// same transaction.
func TestCreateAccountStoresNormalizedAddressAndHash(t *testing.T) {
	f := newFixture(t)

	user, err := f.accounts.Create(t.Context(), "  Ada@Example.TEST ", "", "correct horse battery")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if user.EmailNormalized != "ada@example.test" {
		t.Errorf("address = %q, want the normalized form", user.EmailNormalized)
	}
	if user.DisplayName != "ada@example.test" {
		t.Errorf("display name = %q, want the address when none is given", user.DisplayName)
	}
	if user.Status != UserActive || user.PrincipalID.IsZero() || user.ID.IsZero() {
		t.Errorf("new user = %+v, want an active user with identities", user)
	}
	creds, err := f.store.GetUserCredentialsByEmail(t.Context(), "ada@example.test")
	if err != nil {
		t.Fatalf("stored credentials: %v", err)
	}
	if creds.PasswordHash != "hash:correct horse battery" {
		t.Errorf("stored hash = %q, want the hash of the password", creds.PasswordHash)
	}
	created := f.store.entriesWith("account.created")
	if len(created) != 1 || created[0].Outcome != audit.OutcomeSuccess {
		t.Fatalf("account.created entries = %d, want one successful entry", len(created))
	}
	if created[0].Actor.Kind != audit.ActorSystem {
		t.Errorf("actor kind = %q, want the installation (SYSTEM)", created[0].Actor.Kind)
	}
}

// TestCreateAccountUsesOneSystemPrincipal: the installation acts through one SYSTEM principal,
// created the first time it is needed and reused after that.
func TestCreateAccountUsesOneSystemPrincipal(t *testing.T) {
	f := newFixture(t)
	if _, err := f.accounts.Create(t.Context(), "ada@example.test", "Ada", "correct horse battery"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	first, err := f.store.GetSystemPrincipal(t.Context())
	if err != nil {
		t.Fatalf("system principal: %v", err)
	}
	if _, err := f.accounts.Create(t.Context(), "bob@example.test", "Bob", "correct horse battery"); err != nil {
		t.Fatalf("second create: %v", err)
	}
	second, err := f.store.GetSystemPrincipal(t.Context())
	if err != nil {
		t.Fatalf("system principal after second create: %v", err)
	}
	if first.ID != second.ID {
		t.Error("the installation got a second SYSTEM principal")
	}
}

// TestCreateAccountAdoptsSystemPrincipalCreatedByAnotherWriter: when a concurrent writer
// creates the SYSTEM principal first, the conflict is not an error; the account uses theirs.
func TestCreateAccountAdoptsSystemPrincipalCreatedByAnotherWriter(t *testing.T) {
	f := newFixture(t)
	f.store.raceSystemPrincipal = true

	if _, err := f.accounts.Create(t.Context(), "ada@example.test", "Ada", "correct horse battery"); err != nil {
		t.Fatalf("create while another writer holds the system principal: %v", err)
	}
	system, err := f.store.GetSystemPrincipal(t.Context())
	if err != nil {
		t.Fatalf("system principal: %v", err)
	}
	if got := f.store.entriesWith("account.created"); len(got) != 1 || got[0].Actor.PrincipalID != id.UUID(system.ID) {
		t.Error("the creation is not audited under the system principal the other writer created")
	}
}

func TestCreateAccountRejectsInvalidInput(t *testing.T) {
	cases := map[string]struct {
		email    string
		password string
	}{
		"empty address":        {email: "   ", password: "correct horse battery"},
		"address without @":    {email: "ada.example.test", password: "correct horse battery"},
		"address over 320":     {email: strings.Repeat("a", 316) + "@x.io", password: "correct horse battery"},
		"password under 12":    {email: "ada@example.test", password: "eleven-char"},
		"password over 1024 B": {email: "ada@example.test", password: strings.Repeat("p", 1025)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.accounts.Create(t.Context(), tc.email, "", tc.password)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
			if len(f.store.users) != 0 {
				t.Error("a rejected account was stored")
			}
		})
	}
}

// TestPasswordBoundsCountCharactersNotBytes (SPEC-004 R-AUTH-01): twelve characters are
// enough even when they are several bytes each; the byte cap still applies.
func TestPasswordBoundsCountCharactersNotBytes(t *testing.T) {
	twelveMultibyte := strings.Repeat("ñ", 12) // 12 characters, 24 bytes
	if err := ValidatePassword(twelveMultibyte); err != nil {
		t.Errorf("12 multibyte characters rejected: %v", err)
	}
	if err := ValidatePassword(strings.Repeat("ñ", 11)); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("11 characters accepted: %v", err)
	}
	if err := ValidatePassword(strings.Repeat("ñ", 600)); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("1200 bytes accepted: %v", err)
	}
	if err := ValidatePassword(strings.Repeat("p", 1024)); err != nil {
		t.Errorf("exactly 1024 bytes rejected: %v", err)
	}
}

func TestCreateAccountDuplicateAddressIsConflict(t *testing.T) {
	f := newFixture(t)
	if _, err := f.accounts.Create(t.Context(), "ada@example.test", "", "correct horse battery"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := f.accounts.Create(t.Context(), "ADA@example.test", "", "another long password")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate address: %v, want ErrConflict", err)
	}
	if got := len(f.store.users); got != 1 {
		t.Errorf("users = %d, want the duplicate not stored", got)
	}
}

func TestCreateAccountReportsStorageAndHashFailures(t *testing.T) {
	t.Run("hash fails", func(t *testing.T) {
		f := newFixture(t)
		f.hasher.errHash = errors.New("out of memory")
		_, err := f.accounts.Create(t.Context(), "ada@example.test", "", "correct horse battery")
		if err == nil || errors.Is(err, ErrInvalidInput) {
			t.Fatalf("err = %v, want the hashing failure", err)
		}
	})
	t.Run("system principal lookup fails", func(t *testing.T) {
		f := newFixture(t)
		f.store.errGetSystemPrincipal = errors.New("connection reset")
		if _, err := f.accounts.Create(t.Context(), "ada@example.test", "", "correct horse battery"); err == nil {
			t.Fatal("a failing lookup was ignored")
		}
	})
	t.Run("system principal cannot be created", func(t *testing.T) {
		f := newFixture(t)
		f.store.errCreatePrincipal = errors.New("disk full")
		if _, err := f.accounts.Create(t.Context(), "ada@example.test", "", "correct horse battery"); err == nil {
			t.Fatal("a failing principal insert was ignored")
		}
	})
	t.Run("store refuses the account", func(t *testing.T) {
		f := newFixture(t)
		f.store.errCreateUserEntry = errors.New("write failed")
		if _, err := f.accounts.Create(t.Context(), "ada@example.test", "", "correct horse battery"); err == nil {
			t.Fatal("a failing store write was ignored")
		}
	})
}

// TestSetPasswordReplacesHashAndRevokesSessions (SPEC-016 SH5): the new password signs in,
// the old one does not, and every live session of the account ends with the change.
func TestSetPasswordReplacesHashAndRevokesSessions(t *testing.T) {
	f := newFixture(t)
	user := f.account("ada@example.test", "correct horse battery")
	signed := f.signIn(t, "ada@example.test", "correct horse battery")

	if err := f.accounts.SetPassword(t.Context(), " ADA@example.test", "a brand new password"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	creds, err := f.store.GetUserCredentialsByEmail(t.Context(), "ada@example.test")
	if err != nil {
		t.Fatalf("credentials: %v", err)
	}
	if creds.User.ID != user.ID || creds.PasswordHash != "hash:a brand new password" {
		t.Errorf("stored hash = %q, want the new password's hash", creds.PasswordHash)
	}
	for _, s := range f.store.sessionsOf(user.ID) {
		if s.RevokedAt.IsZero() {
			t.Error("a live session survived the password change")
		}
	}
	if _, err := f.sessions.Authenticate(t.Context(), Credentials{Session: signed.Token}); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("the old session still authenticates: %v", err)
	}
	if got := f.store.entriesWith("password.changed"); len(got) != 1 {
		t.Errorf("password.changed entries = %d, want one", len(got))
	}
}

func TestSetPasswordRejectsUnknownAccountAndShortPassword(t *testing.T) {
	f := newFixture(t)
	f.account("ada@example.test", "correct horse battery")

	if err := f.accounts.SetPassword(t.Context(), "nobody@example.test", "a brand new password"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown account: %v, want ErrNotFound", err)
	}
	if err := f.accounts.SetPassword(t.Context(), "ada@example.test", "short"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("short password: %v, want ErrInvalidInput", err)
	}
	creds, _ := f.store.GetUserCredentialsByEmail(t.Context(), "ada@example.test")
	if creds.PasswordHash != "hash:correct horse battery" {
		t.Error("a rejected change replaced the password")
	}
}

func TestSetPasswordReportsStorageFailures(t *testing.T) {
	f := newFixture(t)
	f.account("ada@example.test", "correct horse battery")
	f.store.errChangePassword = errors.New("write failed")
	if err := f.accounts.SetPassword(t.Context(), "ada@example.test", "a brand new password"); err == nil {
		t.Fatal("a failing change was reported as success")
	}
}

// TestSetDisplayNameRenamesAnAccountUnderTheSystemPrincipal (BL-132): the operator renames an
// account by address; the account.renamed entry belongs to the installation, the name is
// trimmed, an unknown address is not found and a blank name changes nothing.
func TestSetDisplayNameRenamesAnAccountUnderTheSystemPrincipal(t *testing.T) {
	f := newFixture(t)
	user := f.account("owner@example.test", "correct horse battery")

	if err := f.accounts.SetDisplayName(t.Context(), " Owner@Example.test ", "  Juan Tamayo "); err != nil {
		t.Fatalf("set name: %v", err)
	}
	if got := f.store.users[user.ID].DisplayName; got != "Juan Tamayo" {
		t.Errorf("display name = %q, want the trimmed name", got)
	}
	system, err := f.store.GetSystemPrincipal(t.Context())
	if err != nil {
		t.Fatalf("system principal: %v", err)
	}
	entries := f.store.entriesWith("account.renamed")
	if len(entries) != 1 || entries[0].Actor.PrincipalID != id.UUID(system.ID) || entries[0].Actor.Kind != audit.ActorSystem {
		t.Fatalf("account.renamed entries = %+v, want one under the system principal", entries)
	}

	if err := f.accounts.SetDisplayName(t.Context(), "nobody@example.test", "Nobody"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown address: %v, want ErrNotFound", err)
	}
	if err := f.accounts.SetDisplayName(t.Context(), "owner@example.test", "   "); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("blank name: %v, want ErrInvalidInput", err)
	}
	if got := f.store.users[user.ID].DisplayName; got != "Juan Tamayo" {
		t.Errorf("a refused rename changed the name to %q", got)
	}
}
