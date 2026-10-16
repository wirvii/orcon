// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

var wireNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func TestToolNameSanitizing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain name is unchanged", in: "lookup_order", want: "lookup_order"},
		{name: "dot becomes underscore", in: "crm.lookup", want: "crm_lookup"},
		{name: "space and slash become underscores", in: "send mail/now", want: "send_mail_now"},
		{name: "non-ASCII letters become underscores", in: "pedido·ñ", want: "pedido__"},
		{name: "hyphen is kept", in: "get-price", want: "get-price"},
		{name: "long name is truncated to 64", in: strings.Repeat("a", 80), want: strings.Repeat("a", 64)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SanitizeToolName(tt.in)
			if got != tt.want {
				t.Errorf("SanitizeToolName(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if !wireNamePattern.MatchString(got) {
				t.Errorf("SanitizeToolName(%q) = %q, does not match the provider pattern", tt.in, got)
			}
		})
	}
}

func TestToolNamesRoundTrip(t *testing.T) {
	t.Parallel()
	names, err := NewToolNames([]Tool{{Name: "crm.lookup"}, {Name: "crm_lookup_v2"}})
	if err != nil {
		t.Fatalf("NewToolNames: unexpected error: %v", err)
	}
	wire, ok := names.Wire("crm.lookup")
	if !ok || wire != "crm_lookup" {
		t.Errorf("Wire(crm.lookup) = %q, %v; want crm_lookup, true", wire, ok)
	}
	canon, ok := names.Canonical("crm_lookup")
	if !ok || canon != "crm.lookup" {
		t.Errorf("Canonical(crm_lookup) = %q, %v; want crm.lookup, true", canon, ok)
	}
	if _, ok := names.Canonical("unknown_tool"); ok {
		t.Errorf("Canonical(unknown_tool) found a tool that was never declared")
	}
}

func TestToolNamesCollision(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		tools []Tool
	}{
		{name: "dot and underscore collide", tools: []Tool{{Name: "a.b"}, {Name: "a_b"}}},
		{name: "same name twice is a collision of two declarations", tools: []Tool{{Name: "x y"}, {Name: "x_y"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewToolNames(tt.tools)
			if !errors.Is(err, ErrToolNameCollision) {
				t.Errorf("NewToolNames(%v) error = %v, want ErrToolNameCollision", tt.tools, err)
			}
		})
	}
}

func TestToolNamesRejectsEmptyName(t *testing.T) {
	t.Parallel()
	if _, err := NewToolNames([]Tool{{Name: ""}}); err == nil {
		t.Error("NewToolNames with an empty name: error = nil, want an error")
	}
}
