// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity_test

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

func TestCredentialsNeverPrintTheirSecrets(t *testing.T) {
	//nolint:gosec // the values are test strings, not credentials
	creds := identity.Credentials{Bearer: "bearer-secret-value", Session: "session-secret-value", CSRFToken: "csrf-secret-value"}
	for name, got := range map[string]string{
		"%v":  fmt.Sprintf("%v", creds),
		"%+v": fmt.Sprintf("%+v", creds),
		"%#v": fmt.Sprintf("%#v", creds),
		"%s":  creds.String(),
	} {
		if strings.Contains(got, "secret-value") {
			t.Errorf("%s leaks a secret: %q", name, got)
		}
	}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("request", "credentials", creds)
	if strings.Contains(buf.String(), "secret-value") || !strings.Contains(buf.String(), "[REDACTED]") {
		t.Errorf("the structured log leaks or does not redact: %s", buf.String())
	}
}

func TestCredentialsPresent(t *testing.T) {
	tests := []struct {
		creds identity.Credentials
		want  bool
	}{
		{identity.Credentials{}, false},
		{identity.Credentials{CSRFToken: "x", OrgHint: "y"}, false},
		{identity.Credentials{Bearer: "b"}, true},
		{identity.Credentials{Session: "s"}, true},
	}
	for _, tt := range tests {
		if got := tt.creds.Present(); got != tt.want {
			t.Errorf("Present(%+v) = %v, want %v", tt.creds, got, tt.want)
		}
	}
}

func TestRejectAllAuthenticatesNobody(t *testing.T) {
	var a identity.Authenticator = identity.RejectAll{}
	p, err := a.Authenticate(t.Context(), identity.Credentials{Bearer: "valid-looking"})
	if !errors.Is(err, identity.ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
	if !p.IsZero() {
		t.Errorf("principal = %+v, want the zero principal", p)
	}
}

func TestPrincipalIsZero(t *testing.T) {
	if !(identity.Principal{}).IsZero() {
		t.Error("the empty principal is not zero")
	}
	var u id.UUID
	u[15] = 1
	if (identity.Principal{ID: identity.PrincipalID(u)}).IsZero() {
		t.Error("a principal with an id is zero")
	}
}
