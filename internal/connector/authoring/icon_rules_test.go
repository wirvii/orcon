// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authoring

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

const (
	realMark = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M1 1h22v22H1z"/></svg>`
	source   = "simple-icons@16.35.0:whatsapp\n"
)

// loadRegister reads the placeholder register that the lint uses.
func loadRegister(t *testing.T) Placeholders {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("testdata", "placeholder-icons.txt"))
	if err != nil {
		t.Fatalf("read the placeholder register: %v", err)
	}
	set, err := ParsePlaceholders(string(text))
	if err != nil {
		t.Fatalf("parse the placeholder register: %v", err)
	}
	return set
}

func rulesOf(findings []Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Rule)
	}
	return out
}

func hasRule(findings []Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func TestIconMissingFails(t *testing.T) {
	pkg := fstest.MapFS{
		"orcon-package.json": {Data: []byte("{}")},
		"icons/SOURCE":       {Data: []byte(source)},
	}
	got := CheckIcon(pkg, loadRegister(t))
	if !hasRule(got, RuleIconMissing) {
		t.Fatalf("a package without icons/icon.svg must fail %s; got %v", RuleIconMissing, rulesOf(got))
	}
	if len(got) != 1 {
		t.Errorf("a missing icon is the only finding; got %v", rulesOf(got))
	}
}

func TestIconSourceMissingFails(t *testing.T) {
	cases := map[string]fstest.MapFS{
		"no SOURCE file": {
			"icons/icon.svg": {Data: []byte(realMark)},
		},
		"SOURCE names no origin": {
			"icons/icon.svg": {Data: []byte(realMark)},
			"icons/SOURCE":   {Data: []byte("the logo\n")},
		},
		"SOURCE has two lines": {
			"icons/icon.svg": {Data: []byte(realMark)},
			"icons/SOURCE":   {Data: []byte("simple-icons@16.35.0:whatsapp\nextra\n")},
		},
	}
	for name, pkg := range cases {
		t.Run(name, func(t *testing.T) {
			if got := CheckIcon(pkg, loadRegister(t)); !hasRule(got, RuleIconSourceMissing) {
				t.Fatalf("want %s; got %v", RuleIconSourceMissing, rulesOf(got))
			}
		})
	}
}

func TestIconPlaceholderFails(t *testing.T) {
	placeholderBytes := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M0 0"/></svg>`)
	sum := sha256.Sum256(placeholderBytes)
	register, err := ParsePlaceholders(hex.EncodeToString(sum[:]) + " test placeholder\n")
	if err != nil {
		t.Fatalf("parse the test register: %v", err)
	}
	pkg := fstest.MapFS{
		"icons/icon.svg": {Data: placeholderBytes},
		"icons/SOURCE":   {Data: []byte(source)},
	}
	got := CheckIcon(pkg, register)
	if !hasRule(got, RuleIconPlaceholder) {
		t.Fatalf("an icon listed in the register must fail %s; got %v", RuleIconPlaceholder, rulesOf(got))
	}
	if len(got) != 1 {
		t.Errorf("the placeholder is the only finding; got %v", rulesOf(got))
	}
}

func TestIconWithSourceAndRealMarkPasses(t *testing.T) {
	pkg := fstest.MapFS{
		"icons/icon.svg": {Data: []byte(realMark)},
		"icons/SOURCE":   {Data: []byte(source)},
	}
	if got := CheckIcon(pkg, loadRegister(t)); len(got) != 0 {
		t.Fatalf("a real mark with a valid source must pass; got %v", rulesOf(got))
	}
}

// TestOfficialConnectorIcons applies the icon rules to every official connector package
// of the tree: a directory under internal/adapter/connectors that carries an
// orcon-package.json. Built-in node types (core) have no package and keep Orcon glyphs.
func TestOfficialConnectorIcons(t *testing.T) {
	root := filepath.Join("..", "..", "adapter", "connectors")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read the connectors directory: %v", err)
	}
	register := loadRegister(t)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "orcon-package.json")); err != nil {
			continue
		}
		if got := CheckIcon(os.DirFS(dir), register); len(got) != 0 {
			t.Errorf("connector %s: %v", entry.Name(), got)
		}
	}
}
