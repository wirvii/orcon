// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
)

func TestNewCatalog(t *testing.T) {
	cat, err := authz.NewCatalog(map[authz.Scope][]authz.Permission{
		authz.ScopeOrganization: {"org:read", "member:update-role"},
		authz.ScopeProject:      {"flow:read"},
		authz.ScopeInstance:     {"instance:ai-prices"},
	})
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	if !cat.Has("flow:read") || cat.Has("flow:write") {
		t.Error("Has answers wrong")
	}
	if scope, ok := cat.ScopeOf("org:read"); !ok || scope != authz.ScopeOrganization {
		t.Errorf("ScopeOf(org:read) = %q, %v", scope, ok)
	}
	if _, ok := cat.ScopeOf("nope:nope"); ok {
		t.Error("ScopeOf found a permission that is not in the catalog")
	}
	want := []authz.Permission{"flow:read", "instance:ai-prices", "member:update-role", "org:read"}
	if got := cat.Names(); !slices.Equal(got, want) {
		t.Errorf("Names = %v, want %v", got, want)
	}
}

func TestNewCatalogRejectsMalformedDefinitions(t *testing.T) {
	tests := map[string]map[authz.Scope][]authz.Permission{
		"unknown scope":        {"galaxy": {"a:b"}},
		"no colon":             {authz.ScopeProject: {"flowread"}},
		"empty resource":       {authz.ScopeProject: {":read"}},
		"empty action":         {authz.ScopeProject: {"flow:"}},
		"upper case":           {authz.ScopeProject: {"Flow:read"}},
		"leading digit":        {authz.ScopeProject: {"1flow:read"}},
		"underscore":           {authz.ScopeProject: {"flow:read_all"}},
		"two colons":           {authz.ScopeProject: {"a:b:c"}},
		"duplicate in a scope": {authz.ScopeProject: {"flow:read", "flow:read"}},
		"duplicate across":     {authz.ScopeProject: {"flow:read"}, authz.ScopeOrganization: {"flow:read"}},
		"empty name":           {authz.ScopeProject: {""}},
		"space in the action":  {authz.ScopeProject: {"flow:read all"}},
	}
	for name, def := range tests {
		_, err := authz.NewCatalog(def)
		if !errors.Is(err, authz.ErrInvalidCatalog) {
			t.Errorf("%s: err = %v, want ErrInvalidCatalog", name, err)
		}
	}
}

func TestDenyAllDeniesEverything(t *testing.T) {
	var a authz.Authorizer = authz.DenyAll{}
	for _, mode := range []authz.Mode{authz.ModeAll, authz.ModeAny, authz.ModeAuthenticated, authz.ModeAnonymous} {
		got, err := a.Authorize(t.Context(), authz.Request{
			Principal:   identity.Principal{Kind: identity.PrincipalSystem},
			Requirement: authz.Requirement{Mode: mode, Permissions: []authz.Permission{"org:delete"}},
		})
		if err != nil || got != authz.DecisionForbidden {
			t.Errorf("mode %d: DenyAll = %v, %v; want forbidden", mode, got, err)
		}
	}
}

func TestRequirementNeedsDecision(t *testing.T) {
	for mode, want := range map[authz.Mode]bool{
		authz.ModeAll: true, authz.ModeAny: true, authz.ModeAuthenticated: false, authz.ModeAnonymous: false,
	} {
		if got := (authz.Requirement{Mode: mode}).NeedsDecision(); got != want {
			t.Errorf("mode %d: NeedsDecision = %v, want %v", mode, got, want)
		}
	}
}

func TestCatalogSourceIsACopy(t *testing.T) {
	first := authz.CatalogSource()
	if len(first) == 0 {
		t.Fatal("the embedded catalog is empty")
	}
	first[0] ^= 0xff
	if second := authz.CatalogSource(); second[0] == first[0] {
		t.Error("CatalogSource shares its backing array with the caller")
	}
}
