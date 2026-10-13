// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package permissionsyaml_test

import (
	"errors"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/codec/permissionsyaml"
	"github.com/wirvii/orcon/internal/authz"
)

func TestParseTheEmbeddedCatalog(t *testing.T) {
	cat, err := permissionsyaml.Parse(authz.CatalogSource())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for name, scope := range map[authz.Permission]authz.Scope{
		"org:read":          authz.ScopeOrganization,
		"apikey:manage-org": authz.ScopeOrganization,
		"flow:execute":      authz.ScopeProject,
		"git:sync":          authz.ScopeProject,
		"instance:orgs":     authz.ScopeInstance,
	} {
		if got, ok := cat.ScopeOf(name); !ok || got != scope {
			t.Errorf("ScopeOf(%s) = %q, %v; want %q", name, got, ok, scope)
		}
	}
}

// TestCatalogMatchesThePublishedPage keeps internal/authz/permissions.yaml and
// docs/reference/permissions.md in step: the page says it is generated from the
// file once the file exists and must match it.
func TestCatalogMatchesThePublishedPage(t *testing.T) {
	cat, err := permissionsyaml.Parse(authz.CatalogSource())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	page, err := os.ReadFile("../../../../docs/reference/permissions.md")
	if err != nil {
		t.Fatalf("read the published page: %v", err)
	}
	// Permission names in backticks; the role table mentions permissions in
	// running text, so only the three scope sections define the catalog.
	section := string(page)
	if i := strings.Index(section, "## Predefined roles"); i >= 0 {
		section = section[:i]
	}
	defined := map[authz.Permission]bool{}
	for _, m := range regexp.MustCompile("`([a-z][a-z0-9-]*:[a-z][a-z0-9-]*)`").FindAllStringSubmatch(section, -1) {
		defined[authz.Permission(m[1])] = true
	}
	delete(defined, "resource:action") // the form of a name, quoted in the page's introduction
	var fromPage []authz.Permission
	for n := range defined {
		fromPage = append(fromPage, n)
	}
	sort.Slice(fromPage, func(i, j int) bool { return fromPage[i] < fromPage[j] })
	if got := cat.Names(); !slices.Equal(got, fromPage) {
		t.Errorf("permissions.yaml and docs/reference/permissions.md differ:\nyaml only: %v\npage only: %v",
			difference(got, fromPage), difference(fromPage, got))
	}
}

func difference(a, b []authz.Permission) []authz.Permission {
	var out []authz.Permission
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

func TestParseRejectsWhatTheStrictFormatForbids(t *testing.T) {
	tests := map[string]string{
		"empty file":       "",
		"no version":       "permissions:\n  project:\n    - name: flow:read\n",
		"wrong version":    "version: 2\npermissions:\n  project:\n    - name: flow:read\n",
		"unknown key":      "version: 1\nroles: {}\npermissions:\n  project:\n    - name: flow:read\n",
		"unknown scope":    "version: 1\npermissions:\n  galaxy:\n    - name: flow:read\n",
		"item no name":     "version: 1\npermissions:\n  project:\n    - note: orphan\n",
		"bad name":         "version: 1\npermissions:\n  project:\n    - name: Flow-Read\n",
		"duplicate":        "version: 1\npermissions:\n  project:\n    - name: flow:read\n  organization:\n    - name: flow:read\n",
		"not a mapping":    "- just\n- a list\n",
		"scalar items":     "version: 1\npermissions:\n  project:\n    - flow:read\n",
		"unknown item key": "version: 1\npermissions:\n  project:\n    - name: flow:read\n      grants: [x]\n",
	}
	for name, doc := range tests {
		_, err := permissionsyaml.Parse([]byte(doc))
		if !errors.Is(err, permissionsyaml.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestParseAcceptsNotes(t *testing.T) {
	cat, err := permissionsyaml.Parse([]byte("version: 1\npermissions:\n  project:\n    - name: flow:read\n      note: Reads a flow.\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !cat.Has("flow:read") {
		t.Error("flow:read is missing")
	}
}
