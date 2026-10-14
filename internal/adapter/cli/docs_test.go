// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/cli"
	"github.com/wirvii/orcon/internal/platform/config"
)

func TestGenerateDocs(t *testing.T) {
	t.Parallel()

	files, err := cli.GenerateDocs(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = string(f.Content)
	}
	for _, p := range []string{"configuration.md", "es/configuration.md", "configuration.schema.json"} {
		if byPath[p] == "" {
			t.Fatalf("%s was not generated", p)
		}
	}
	en, es := byPath["configuration.md"], byPath["es/configuration.md"]
	for _, k := range config.Default().Keys() {
		for name, page := range map[string]string{"en": en, "es": es} {
			if !strings.Contains(page, "#### `"+k.Path+"`") || !strings.Contains(page, "`"+k.EnvName()+"`") {
				t.Errorf("%s reference lacks key %s or its environment variable", name, k.Path)
			}
		}
		if k.Secret && (!strings.Contains(en, k.EnvFileName()) || !strings.Contains(es, "--"+k.FlagFileName())) {
			t.Errorf("secret %s does not document its file variant", k.Path)
		}
	}
	for _, want := range []string{"Configuration reference", "SPDX-License-Identifier: Apache-2.0", "[REDACTED]"} {
		if !strings.Contains(en, want) {
			t.Errorf("English reference lacks %q", want)
		}
	}
	for _, want := range []string{"Referencia de configuración", "Cuándo toma efecto", "[REDACTED]", "al reiniciar"} {
		if !strings.Contains(es, want) {
			t.Errorf("Spanish reference lacks %q", want)
		}
	}
	if strings.Contains(es, "Takes effect") || strings.Contains(en, "Cuándo") {
		t.Error("languages are mixed")
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(byPath["configuration.schema.json"]), &schema); err != nil {
		t.Fatalf("schema is not JSON: %v", err)
	}
	if !strings.Contains(byPath["configuration.schema.json"], "SPDX-License-Identifier: Apache-2.0") {
		t.Error("the schema must carry its license in $comment")
	}
}

func TestGenerateDocsIsDeterministic(t *testing.T) {
	t.Parallel()

	a, err := cli.GenerateDocs(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	b, err := cli.GenerateDocs(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	for i := range a {
		if !bytes.Equal(a[i].Content, b[i].Content) {
			t.Errorf("%s differs between runs", a[i].Path)
		}
	}
}

func TestDocsGenerateWritesTheFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := exec(t, cli.Env{}, "docs", "generate", "--out", dir)
	if r.code != cli.ExitOK {
		t.Fatalf("exit = %d: %s", r.code, r.err.String())
	}
	for _, p := range []string{"configuration.md", "es/configuration.md", "configuration.schema.json"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	if r := exec(t, cli.Env{}, "docs"); r.code != cli.ExitUsage {
		t.Errorf("docs without a command: exit = %d", r.code)
	}
	if r := exec(t, cli.Env{}, "docs", "generate", "--bogus"); r.code != cli.ExitUsage {
		t.Errorf("bad flag: exit = %d", r.code)
	}
}
