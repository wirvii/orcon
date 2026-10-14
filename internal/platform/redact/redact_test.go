// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package redact_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/platform/redact"
)

func TestSecretNeverPrints(t *testing.T) {
	t.Parallel()

	const plain = "hunter2-the-secret"
	s := redact.New(plain)
	if !s.IsSet() || s.Reveal() != plain {
		t.Fatalf("Reveal = %q", s.Reveal())
	}
	var logged bytes.Buffer
	slog.New(slog.NewJSONHandler(&logged, nil)).Info("test", "secret", s)
	text, err := s.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	asJSON, err := s.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]string{
		"String":    s.String(),
		"%v":        fmt.Sprintf("%v", s),
		"%+v":       fmt.Sprintf("%+v", s),
		"%#v":       fmt.Sprintf("%#v", s),
		"%q":        fmt.Sprintf("%q", s),
		"%x":        fmt.Sprintf("%x", s),
		"%d":        fmt.Sprintf("%d", s),
		"Sprint":    fmt.Sprint(s),
		"struct":    fmt.Sprintf("%+v", struct{ S redact.Secret }{s}),
		"pointer":   fmt.Sprintf("%v", &s),
		"slog JSON": logged.String(),
		"text":      string(text),
		"JSON":      string(asJSON),
	} {
		if strings.Contains(got, plain) {
			t.Errorf("%s leaks the secret: %q", name, got)
		}
		if !strings.Contains(got, redact.Redacted) {
			t.Errorf("%s = %q, want it to show %s", name, got, redact.Redacted)
		}
	}
	var zero redact.Secret
	if zero.IsSet() || zero.Reveal() != "" {
		t.Error("the zero Secret must be empty")
	}
}
