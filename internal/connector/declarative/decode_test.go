// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"testing"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// nestedBody has two holders with arrays, so a wildcard fan-out is observable.
const nestedBody = `{"entry":[{"changes":[{"value":{"tag":"a","items":[{"n":1},{"n":2}]}},{"value":{"tag":"b","items":[{"n":3}]}}]}]}`

func TestDecodeFansOutEveryItemWithItsScopes(t *testing.T) {
	d := Decode{
		Items: "$.entry[*].changes[*].value.items[*]",
		Map: map[string]string{
			"n":   "$.item.n",
			"tag": "$.parent.tag",
			"ok":  "$.root.entry[0].changes[0].value.tag",
		},
	}
	events, err := d.Events(mustParse(t, []byte(nestedBody)))
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	want := []string{
		`{"n":1,"ok":"a","tag":"a"}`,
		`{"n":2,"ok":"a","tag":"a"}`,
		`{"n":3,"ok":"a","tag":"b"}`,
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d", len(events), len(want))
	}
	for i := range want {
		got, _ := jsonvalue.Canonical(events[i])
		if string(got) != want[i] {
			t.Errorf("event %d = %s, want %s", i, got, want[i])
		}
	}
}

func TestDecodeAlternativesTakeTheFirstFound(t *testing.T) {
	d := Decode{Items: "$.list[*]", Map: map[string]string{"v": "$.item.a|$.item.b|$.item.c"}}
	events, err := d.Events(mustParse(t, []byte(`{"list":[{"b":2},{"a":1,"b":3},{"x":0}]}`)))
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	wants := []string{`{"v":2}`, `{"v":1}`, `{}`}
	for i, w := range wants {
		got, _ := jsonvalue.Canonical(events[i])
		if string(got) != w {
			t.Errorf("event %d = %s, want %s", i, got, w)
		}
	}
}

func TestDecodeWithoutTheArrayYieldsNothing(t *testing.T) {
	d := Decode{Items: "$.entry[*].changes[*].value.messages[*]", Map: map[string]string{"id": "$.item.id"}}
	events, err := d.Events(mustParse(t, []byte(`{"entry":[{"changes":[{"value":{"statuses":[]}}]}]}`)))
	if err != nil || len(events) != 0 {
		t.Fatalf("Events = %v, %v; want no events and no error", events, err)
	}
}

func TestValidateDecodeDeclarations(t *testing.T) {
	cases := map[string]Decode{
		"items without a wildcard":     {Items: "$.messages", Map: map[string]string{"id": "$.item.id"}},
		"items with no array name":     {Items: "$[*]", Map: map[string]string{"id": "$.item.id"}},
		"empty map":                    {Items: "$.messages[*]"},
		"map reads outside the scopes": {Items: "$.messages[*]", Map: map[string]string{"id": "$.id"}},
		"map with a wildcard":          {Items: "$.messages[*]", Map: map[string]string{"id": "$.item.x[*]"}},
		"map alternative not a path":   {Items: "$.messages[*]", Map: map[string]string{"id": "$.item.a|text"}},
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			tr := Trigger{Family: FamilyWebhook, Verify: &Verify{Type: VerifyHMACSHA256, Header: "X-Sig", Field: "s"}, Decode: &d}
			def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": readOp("/items")})
			def.Triggers = map[string]Trigger{"inbound": tr}
			if err := Validate(def); err == nil {
				t.Fatal("Validate accepted the decode declaration")
			}
		})
	}
}
