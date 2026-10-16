// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

func TestIdentityTypes(t *testing.T) {
	u := id.NewSequence().New()
	text := u.String()
	if identity.OrgID(u).String() != text || identity.PrincipalID(u).String() != text || identity.ProjectID(u).String() != text {
		t.Error("String must be the hyphenated form of the underlying UUID")
	}
	if identity.OrgID(u).IsZero() || identity.PrincipalID(u).IsZero() || identity.ProjectID(u).IsZero() {
		t.Error("a set identity is not zero")
	}
	if !(identity.OrgID{}).IsZero() || !(identity.PrincipalID{}).IsZero() || !(identity.ProjectID{}).IsZero() {
		t.Error("the zero identity is zero")
	}
}

// TestUserMembershipAndSessionIdentities covers the identities of the account and
// sign-in rows: each prints the hyphenated UUID and is zero only when unset.
func TestUserMembershipAndSessionIdentities(t *testing.T) {
	u := id.NewSequence().New()
	text := u.String()
	if identity.UserID(u).String() != text || identity.MembershipID(u).String() != text || identity.SessionID(u).String() != text {
		t.Error("String must be the hyphenated form of the underlying UUID")
	}
	if identity.UserID(u).IsZero() || identity.MembershipID(u).IsZero() || identity.SessionID(u).IsZero() {
		t.Error("a set identity is not zero")
	}
	if !(identity.UserID{}).IsZero() || !(identity.MembershipID{}).IsZero() || !(identity.SessionID{}).IsZero() {
		t.Error("the zero identity is zero")
	}
}
