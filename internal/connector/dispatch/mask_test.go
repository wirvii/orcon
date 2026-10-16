// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func parseValue(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(text))
	if err != nil {
		t.Fatalf("parse %s: %v", text, err)
	}
	return v
}

func marshalValue(t *testing.T, v jsonvalue.Value) string {
	t.Helper()
	out, err := jsonvalue.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(out)
}

func TestMaskerReplacesSecretsInValuesAndKeys(t *testing.T) {
	m := newMasker(Credential{Mask: []string{"tok-123"}})
	in := parseValue(t, `{"echo":"Bearer tok-123","tok-123":1,"list":["x tok-123 y",{"deep tok-123":"tok-123"}],"n":7,"ok":"fine"}`)
	got := marshalValue(t, m.items([]jsonvalue.Value{in})[0])
	if strings.Contains(got, "tok-123") {
		t.Fatalf("the masked items still hold the secret: %s", got)
	}
	for _, keep := range []string{`"n":7`, `"ok":"fine"`, "Bearer [REDACTED]"} {
		if !strings.Contains(got, keep) {
			t.Errorf("masked items lost %q: %s", keep, got)
		}
	}
}

func TestMaskerKeepsCollidingKeysApart(t *testing.T) {
	m := newMasker(Credential{Mask: []string{"aaa"}})
	got := marshalValue(t, m.value(parseValue(t, `{"aaa":1,"[REDACTED]":2}`)))
	if strings.Count(got, "REDACTED") != 2 || strings.Contains(got, "aaa") {
		t.Fatalf("colliding keys = %s", got)
	}
}

func TestMaskerCoversTheBasicHeaderValue(t *testing.T) {
	pair := base64.StdEncoding.EncodeToString([]byte("user-x:pass-y"))
	m := newMasker(Credential{
		Auth:   declarative.Auth{Type: declarative.AuthBasic},
		Fields: map[string]string{"username": "user-x", "password": "pass-y"},
		Mask:   []string{"user-x", "pass-y"},
	})
	got := marshalValue(t, m.value(parseValue(t, `{"authorization":"Basic `+pair+`"}`)))
	if strings.Contains(got, pair) || strings.Contains(got, "user-x") {
		t.Fatalf("the base64 header value was not masked: %s", got)
	}
}

func TestMaskerNumericSecretAndNothingToMask(t *testing.T) {
	m := newMasker(Credential{Mask: []string{"4242"}})
	got := marshalValue(t, m.value(parseValue(t, `{"pin":4242,"other":17}`)))
	if strings.Contains(got, "4242") || !strings.Contains(got, `"other":17`) {
		t.Fatalf("numeric masking = %s", got)
	}
	if newMasker(Credential{}) != nil {
		t.Fatal("a credential without secrets builds a masker")
	}
	in := []jsonvalue.Value{parseValue(t, `{"a":1}`)}
	if out := newMasker(Credential{}).items(in); len(out) != 1 || marshalValue(t, out[0]) != `{"a":1}` {
		t.Fatalf("no masker changed the items: %v", out)
	}
}
