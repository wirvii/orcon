// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/platform/config"
)

func TestDefaultSchemaIsValid(t *testing.T) {
	t.Parallel()

	s := config.Default()
	if got := len(s.Keys()); got < 60 {
		t.Fatalf("the schema has %d keys, want the whole set of SPEC-015 section 5.1", got)
	}
	for _, k := range s.Keys() {
		if k.Description.EN == "" || k.Description.ES == "" {
			t.Errorf("%s lacks a description in one language", k.Path)
		}
		if strings.Contains(k.Path, "_") || strings.ContainsAny(k.Path, "- ") {
			t.Errorf("%s is not camelCase", k.Path)
		}
	}
}

func TestEnvNamesUniqueAndNotReserved(t *testing.T) {
	t.Parallel()

	reserved := config.ClientEnvNames()
	for _, want := range []string{"ORCON_URL", "ORCON_TOKEN", "ORCON_CONTEXT", "ORCON_CLIENT_ID", "ORCON_CLIENT_SECRET", "ORCON_FEDERATED_TOKEN_FILE", "ORCON_CONFIG"} {
		if !slices.Contains(reserved, want) {
			t.Errorf("%s is not in the reserved client names", want)
		}
	}
	seen := map[string]string{}
	claim := func(name, owner string) {
		t.Helper()
		if slices.Contains(reserved, name) {
			t.Errorf("%s produces the reserved name %s", owner, name)
		}
		if prev, dup := seen[name]; dup {
			t.Errorf("%s is produced by both %s and %s", name, prev, owner)
		}
		seen[name] = owner
	}
	flags := map[string]string{}
	for _, k := range config.Default().Keys() {
		claim(k.EnvName(), k.Path)
		if !strings.HasPrefix(k.EnvName(), config.EnvPrefix) {
			t.Errorf("%s: %s lacks the ORCON_ prefix", k.Path, k.EnvName())
		}
		if k.Secret {
			claim(k.EnvFileName(), k.Path+" (file)")
			flags[k.FlagFileName()] = k.Path
			continue
		}
		if prev, dup := flags[k.FlagName()]; dup {
			t.Errorf("flag --%s is produced by both %s and %s", k.FlagName(), prev, k.Path)
		}
		flags[k.FlagName()] = k.Path
	}
}

func TestDerivedNames(t *testing.T) {
	t.Parallel()

	tests := []struct{ path, env, flag, fileKey string }{
		{"store.postgres.maxConnections", "ORCON_STORE_POSTGRES_MAX_CONNECTIONS", "store-postgres-max-connections", "store.postgres.maxConnectionsFile"},
		{"role", "ORCON_ROLE", "role", "roleFile"},
		{"blobs.s3.accessKeyId", "ORCON_BLOBS_S3_ACCESS_KEY_ID", "blobs-s3-access-key-id", "blobs.s3.accessKeyIdFile"},
		{"keys.master.awskms.keyArn", "ORCON_KEYS_MASTER_AWSKMS_KEY_ARN", "keys-master-awskms-key-arn", "keys.master.awskms.keyArnFile"},
		{"publicUrl", "ORCON_PUBLIC_URL", "public-url", "publicUrlFile"},
	}
	for _, tt := range tests {
		if got := config.EnvName(tt.path); got != tt.env {
			t.Errorf("EnvName(%q) = %q, want %q", tt.path, got, tt.env)
		}
		if got := config.FlagName(tt.path); got != tt.flag {
			t.Errorf("FlagName(%q) = %q, want %q", tt.path, got, tt.flag)
		}
		if got := config.FilePath(tt.path); got != tt.fileKey {
			t.Errorf("FilePath(%q) = %q, want %q", tt.path, got, tt.fileKey)
		}
	}
}

func TestNewSchemaRejectsBadDeclarations(t *testing.T) {
	t.Parallel()

	desc := config.Text{EN: "d", ES: "d"}
	str := func(path string) config.Key {
		return config.Key{Path: path, Kind: config.KindString, Description: desc}
	}
	secret := func(path string) config.Key {
		return config.Key{Path: path, Kind: config.KindString, Secret: true, Description: desc}
	}
	tests := []struct {
		name  string
		keys  []config.Key
		match string
	}{
		{"duplicate path", []config.Key{str("a.b"), str("a.b")}, "declared twice"},
		{"same environment name", []config.Key{str("a.bC"), {Path: "other", Kind: config.KindString, Env: "ORCON_A_B_C", Description: desc}}, "ORCON_A_B_C is produced by both"},
		{"reserved client name", []config.Key{str("url")}, "ORCON_URL is produced by both"},
		{"reserved client name as a file variant", []config.Key{secret("federatedToken")}, "ORCON_FEDERATED_TOKEN_FILE"},
		{"reserved config name", []config.Key{str("config")}, "ORCON_CONFIG"},
		{"reserved flag", []config.Key{str("probe")}, "flag --probe"},
		{"value and section", []config.Key{str("a"), str("a.b")}, "both a value and a section"},
		{"missing spanish", []config.Key{{Path: "a", Kind: config.KindString, Description: config.Text{EN: "d"}}}, "English and Spanish"},
		{"enum without values", []config.Key{{Path: "a", Kind: config.KindEnum, Description: desc}}, "enum without values"},
		{"secret with a default", []config.Key{{Path: "a", Kind: config.KindString, Secret: true, Default: "x", Description: desc}}, "must not have a default"},
		{"invalid default", []config.Key{{Path: "a", Kind: config.KindInt, Default: "x", Description: desc}}, "default of key a is invalid"},
		{"file variant clashes with a key", []config.Key{secret("a"), str("aFile")}, "ORCON_A_FILE"},
		{"malformed path", []config.Key{str("a..b")}, "malformed path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := config.NewSchema(tt.keys)
			if err == nil || !strings.Contains(err.Error(), tt.match) {
				t.Fatalf("NewSchema = %v, want an error containing %q", err, tt.match)
			}
		})
	}
}

func TestSchemaIndexes(t *testing.T) {
	t.Parallel()

	s := config.Default()
	for path, want := range map[string]bool{
		"http.address": true, "store.postgres.dsn": true, "store.postgres.dsnFile": true,
		"http": false, "store.postgres": false, "nope": false, "http.addressFile": false,
	} {
		if got := s.IsLeaf(path); got != want {
			t.Errorf("IsLeaf(%q) = %v, want %v", path, got, want)
		}
	}
	if !s.IsSection("store.postgres") || s.IsSection("store.postgres.dsn") {
		t.Error("IsSection must be true for a proper prefix only")
	}
	if _, ok := s.Key("http.address"); !ok {
		t.Error("Key(http.address) not found")
	}
	if !slices.Contains(s.EnvNames(), "ORCON_STORE_POSTGRES_DSN_FILE") {
		t.Error("EnvNames lacks a secret file variant")
	}
}

func TestKeyAppliesToRoles(t *testing.T) {
	t.Parallel()

	s := config.Default()
	address, _ := s.Key("http.address")
	if !address.AppliesTo(config.RoleAPI) || address.AppliesTo(config.RoleWorker) {
		t.Error("http.address belongs to api and standalone only")
	}
	level, _ := s.Key("log.level")
	if !level.AppliesTo(config.RoleWorker) {
		t.Error("log.level applies to every role")
	}
}

func TestReloadAndKindNames(t *testing.T) {
	t.Parallel()

	for r, want := range map[config.Reload]string{config.ReloadRestart: "restart", config.ReloadLive: "live", config.ReloadFirstStart: "first start", config.Reload(99): "unknown"} {
		if r.String() != want {
			t.Errorf("Reload(%d) = %q, want %q", r, r.String(), want)
		}
	}
	for k := config.KindString; k <= config.KindMap; k++ {
		if k.String() == "unknown" {
			t.Errorf("Kind(%d) has no name", k)
		}
	}
}
