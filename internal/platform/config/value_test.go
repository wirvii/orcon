// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/platform/config"
)

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

func TestValueAndEntryNeverPrintASecret(t *testing.T) {
	t.Parallel()

	const plain = "hunter2-the-secret"
	res := load(t, input{environ: baseEnv("ORCON_MASTER_KEY", "aHVudGVyMi10aGUtc2VjcmV0", "ORCON_OBSERVABILITY_OTLP_HEADERS", "Authorization="+plain)})
	for _, path := range []string{"keys.master.key", "observability.otlp.headers"} {
		v := res.Config.Value(path)
		var entry config.Entry
		for _, e := range res.Config.Entries() {
			if e.Key.Path == path {
				entry = e
			}
		}
		for name, got := range map[string]string{
			"Value %v":   fmt.Sprintf("%v", v),
			"Value %+v":  fmt.Sprintf("%+v", v),
			"Value %#v":  fmt.Sprintf("%#v", v),
			"Value %s":   fmt.Sprintf("%s", v), //nolint:perfsprint,gocritic // the verb is what is under test
			"Value %x":   fmt.Sprintf("%x", v),
			"Value %d":   fmt.Sprintf("%d", v),
			"Entry %v":   fmt.Sprintf("%v", entry),
			"Entry %+v":  fmt.Sprintf("%+v", entry),
			"Entry %#v":  fmt.Sprintf("%#v", entry),
			"Entry ptr":  fmt.Sprintf("%#v", &entry),
			"Config %#v": fmt.Sprintf("%#v", res.Config),
			"Config %+v": fmt.Sprintf("%+v", res.Config),
		} {
			if strings.Contains(got, plain) || strings.Contains(got, "aHVudGVyMi10aGUtc2VjcmV0") {
				t.Errorf("%s of %s leaks the secret: %q", name, path, got)
			}
		}
	}
}
