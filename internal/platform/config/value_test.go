// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"bytes"
	"encoding"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/platform/config"
)

func TestSecretNeverPrintsItsValue(t *testing.T) {
	t.Parallel()

	const plain = "hunter2-the-secret"
	s := config.NewSecret(plain)
	if !s.IsSet() || s.Reveal() != plain {
		t.Fatalf("Reveal = %q", s.Reveal())
	}
	var logged bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logged, nil))
	logger.Info("test", "secret", s)
	text, err := s.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var _ encoding.TextMarshaler = s
	for name, got := range map[string]string{
		"String":  s.String(),
		"%v":      fmt.Sprintf("%v", s),
		"%+v":     fmt.Sprintf("%+v", s),
		"%#v":     fmt.Sprintf("%#v", s),
		"%s":      fmt.Sprint(s),
		"struct":  fmt.Sprintf("%+v", struct{ S config.Secret }{s}),
		"slog":    logged.String(),
		"marshal": string(text),
	} {
		if strings.Contains(got, plain) {
			t.Errorf("%s leaks the secret: %q", name, got)
		}
		if !strings.Contains(got, config.Redacted) {
			t.Errorf("%s = %q, want it to show %s", name, got, config.Redacted)
		}
	}
	if config.NewSecret("").IsSet() {
		t.Error("an empty secret reports as set")
	}
}

func TestValueEdgeCases(t *testing.T) {
	t.Parallel()

	var zero config.Value
	if !zero.IsZero() || zero.Int() != 0 || zero.Bool() || zero.Duration() != 0 || zero.Bytes() != 0 || zero.Float() != 0 {
		t.Error("the zero Value must read as empty everywhere")
	}
	res := load(t, input{args: []string{"--store-postgres-declared-capacity", "2KiB"}})
	if got := res.Config.Value("store.postgres.declaredCapacity").Bytes(); got != 2048 {
		t.Errorf("Bytes = %d, want 2048", got)
	}
	if got := res.Config.Value("nope"); !got.IsZero() {
		t.Errorf("Value of an unknown path = %v, want zero", got)
	}
	if got := res.Config.Source("nope"); got.String() != "default" {
		t.Errorf("Source of an unknown path = %v", got)
	}
}

func TestSourceStrings(t *testing.T) {
	t.Parallel()

	file := config.Source{Kind: config.SourceFile, Path: "/c.yaml", Line: 3}
	tests := []struct {
		src  config.Source
		want string
	}{
		{config.Source{}, "default"},
		{config.Source{Kind: config.SourceFlag}, "flag"},
		{config.Source{Kind: config.SourceEnv, Env: "ORCON_X"}, "env:ORCON_X"},
		{file, "file:/c.yaml:3"},
		{config.Source{Kind: config.SourceSecretFile, Path: "/run/s", Via: &file}, "file:/run/s (file:/c.yaml:3)"},
		{config.Source{Kind: config.SourceSecretFile, Path: "/run/s"}, "file:/run/s"},
	}
	for _, tt := range tests {
		if got := tt.src.String(); got != tt.want {
			t.Errorf("Source.String() = %q, want %q", got, tt.want)
		}
	}
	if config.SeverityError.String() != "ERROR" || config.SeverityWarning.String() != "WARN" {
		t.Error("severity labels changed")
	}
}
