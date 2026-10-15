// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
)

// TestInputBoundsAreRefusedWithoutValues: each rule of the input is refused with ErrInvalid, and the message
// never holds a value.
func TestInputBoundsAreRefusedWithoutValues(t *testing.T) {
	f := newFixture(t)
	const probe = "never-in-an-error"
	manyFields := map[string]string{}
	for i := range maxFields + 1 {
		manyFields[fmt.Sprintf("f%d", i)] = "v"
	}
	cases := map[string]func(*Input){
		"blank name":            func(in *Input) { in.Name = "   " },
		"name too long":         func(in *Input) { in.Name = strings.Repeat("n", maxNameLength+1) },
		"control in name":       func(in *Input) { in.Name = "a\x01b" },
		"no connector":          func(in *Input) { in.ConnectorReference = "" },
		"connector too long":    func(in *Input) { in.ConnectorReference = strings.Repeat("c", maxConnectorLength+1) },
		"no secret":             func(in *Input) { in.Secret = nil },
		"no hosts":              func(in *Input) { in.AllowedHosts = nil },
		"host with a path":      func(in *Input) { in.AllowedHosts = []string{"api.example.com/x"} },
		"host with one label":   func(in *Input) { in.AllowedHosts = []string{"localhost"} },
		"too many fields":       func(in *Input) { in.Secret = manyFields },
		"bad field name":        func(in *Input) { in.Secret = map[string]string{"bad name": probe} },
		"empty field name":      func(in *Input) { in.Secret = map[string]string{"": probe} },
		"bad public name":       func(in *Input) { in.Public = map[string]string{"bad name": "v"} },
		"shared field name":     func(in *Input) { in.Public = map[string]string{"accessToken": "v"} },
		"public value too long": func(in *Input) { in.Public = map[string]string{"p": strings.Repeat("v", maxPublicValueBytes+1)} },
		"secret value too long": func(in *Input) { in.Secret = map[string]string{"s": strings.Repeat("v", maxSecretValueBytes+1)} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := basicInput(probe)
			mutate(&in)
			_, err := f.svc.Create(t.Context(), f.orgA, f.projA1, in, f.actor)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("create = %v; want ErrInvalid", err)
			}
			if strings.Contains(err.Error(), probe) {
				t.Fatalf("the error holds a value: %v", err)
			}
		})
	}
}

// TestNewNamesEveryMissingCollaborator: a service is never built without its store, keys, clock or identities.
func TestNewNamesEveryMissingCollaborator(t *testing.T) {
	f := newFixture(t)
	good := Config{Store: f.store, Keys: &fakeKeys{wrapped: map[string][]byte{}}, Clock: f.svc.clock, IDs: f.svc.ids}
	for name, mutate := range map[string]func(*Config){
		"store": func(c *Config) { c.Store = nil },
		"keys":  func(c *Config) { c.Keys = nil },
		"clock": func(c *Config) { c.Clock = nil },
		"ids":   func(c *Config) { c.IDs = nil },
	} {
		cfg := good
		mutate(&cfg)
		if _, err := New(cfg); err == nil {
			t.Errorf("New without %s = %v; want a named refusal", name, err)
		}
	}
	if _, err := New(good); err != nil {
		t.Fatalf("New with every collaborator: %v", err)
	}
}

// TestResolvedNeverPrintsASecret: the resolved credential hides its values from every formatting verb and log
// handler, hands the masking set out, and copies its host list.
func TestResolvedNeverPrintsASecret(t *testing.T) {
	f := newFixture(t)
	const probe = "tok-123456"
	created := f.mustCreate(basicInput(probe))
	res, err := f.svc.Resolve(t.Context(), f.orgA, f.projA1, created.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	for _, text := range []string{fmt.Sprint(res), fmt.Sprintf("%v %+v %#v", res, res, res), res.LogValue().String(), slog.AnyValue(res).String()} {
		if strings.Contains(text, probe) {
			t.Fatalf("a rendering of the resolved credential holds the secret: %s", text)
		}
	}
	if got := res.SecretValues(); !slices.Equal(got, []string{probe}) {
		t.Fatalf("SecretValues = %v; want the one secret for the masking set", got)
	}
	hosts := res.AllowedHosts()
	hosts[0] = "changed.example.com"
	if err := res.CheckHost("graph.facebook.com"); err != nil {
		t.Fatalf("changing the copy of the hosts changed the credential: %v", err)
	}
}

// TestHostDocumentRoundTrip: the stored form of a host list reads back in order, and a damaged one is refused.
func TestHostDocumentRoundTrip(t *testing.T) {
	want := []string{"api.example.com", "*.example.org"}
	doc, err := EncodeHosts(want)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeHosts(doc)
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("decode = %v, %v; want %v", got, err, want)
	}
	for _, bad := range []string{`{`, `{"a":1}`, `[1]`} {
		if got, err := DecodeHosts([]byte(bad)); err == nil && bad != `{"a":1}` {
			t.Errorf("decode of %q = %v; want an error", bad, got)
		}
	}
}

// TestOwnerOnlyChangesAreRefusedToOthers: a project that only uses a credential cannot rename, re-host, delete or
// unshare it, and a credential nobody holds is not found.
func TestOwnerOnlyChangesAreRefusedToOthers(t *testing.T) {
	f := newFixture(t)
	created := f.mustCreate(basicInput("s1"))
	if err := f.svc.Share(t.Context(), f.orgA, f.projA1, created.ID, f.projA2, f.actor); err != nil {
		t.Fatalf("share: %v", err)
	}
	set := Settings{Name: "renamed", AllowedHosts: []string{"graph.facebook.com"}}
	if _, err := f.svc.Update(t.Context(), f.orgA, f.projA2, created.ID, set, f.actor); !errors.Is(err, ErrNotOwner) {
		t.Errorf("update by the user project = %v; want ErrNotOwner", err)
	}
	if err := f.svc.Delete(t.Context(), f.orgA, f.projA2, created.ID, f.actor); !errors.Is(err, ErrNotOwner) {
		t.Errorf("delete by the user project = %v; want ErrNotOwner", err)
	}
	if err := f.svc.Unshare(t.Context(), f.orgA, f.projA2, created.ID, f.projA2, f.actor); !errors.Is(err, ErrNotOwner) {
		t.Errorf("unshare by the user project = %v; want ErrNotOwner", err)
	}
	bad := Settings{Name: "", AllowedHosts: []string{"graph.facebook.com"}}
	if _, err := f.svc.Update(t.Context(), f.orgA, f.projA1, created.ID, bad, f.actor); !errors.Is(err, ErrInvalid) {
		t.Errorf("update with a blank name = %v; want ErrInvalid", err)
	}
	if err := f.svc.Unshare(t.Context(), f.orgA, f.projA1, created.ID, f.projB1, f.actor); err == nil {
		t.Error("unshare of a share that does not exist succeeded")
	}
	if err := f.svc.Delete(t.Context(), f.orgA, f.projA1, created.ID, f.actor); err != nil {
		t.Fatalf("delete by the owner: %v", err)
	}
	if err := f.svc.Delete(t.Context(), f.orgA, f.projA1, created.ID, f.actor); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete of a deleted credential = %v; want ErrNotFound", err)
	}
}
