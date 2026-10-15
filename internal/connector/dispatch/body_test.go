// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"testing"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func mustParse(t *testing.T, s string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(s))
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return v
}

// TestRenderBodyReadsEachItem checks that the body of every item is rendered from
// that item: a marker alone keeps the type of its field, a marker inside a string
// is written as text, and a field the item lacks fails the item.
func TestRenderBodyReadsEachItem(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		item    string
		want    string
		wantErr bool
	}{
		{
			name: "whole marker keeps the field value",
			body: `{"to":"{{ $json.from }}","n":"{{ $json.count }}"}`,
			item: `{"from":"573000000000","count":3}`,
			want: `{"to":"573000000000","n":3}`,
		},
		{
			name: "marker inside a longer string is written as text",
			body: `{"text":{"body":"Hola {{ $json.name }}, tu pedido: {{$json.order.id}}"}}`,
			item: `{"name":"Ana","order":{"id":"A-7"}}`,
			want: `{"text":{"body":"Hola Ana, tu pedido: A-7"}}`,
		},
		{
			name: "static values pass through",
			body: `{"type":"text","items":[1,"{{ $json.name }}"]}`,
			item: `{"name":"x"}`,
			want: `{"type":"text","items":[1,"x"]}`,
		},
		{
			name:    "missing field fails the item",
			body:    `{"to":"{{ $json.from }}"}`,
			item:    `{"text":"hi"}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderBody(mustParse(t, tt.body), mustParse(t, tt.item))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("renderBody = %s, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("renderBody: %v", err)
			}
			if got != tt.want {
				t.Fatalf("renderBody = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestCheckMarkersRefusesIncompleteMarkers checks that a "{{" that is not a
// complete {{ $json.<field> }} marker stops the node before anything is sent.
func TestCheckMarkersRefusesIncompleteMarkers(t *testing.T) {
	if err := checkMarkers(mustParse(t, `{"a":"{{ $json.ok }}","b":["plain"]}`)); err != nil {
		t.Fatalf("a complete marker was refused: %v", err)
	}
	for _, body := range []string{`{"a":"{{ $credentials.token }}"}`, `{"a":"{{ $json.x"}`, `{"a":{"b":"{{ 1 + 1 }}"}}`} {
		if err := checkMarkers(mustParse(t, body)); err == nil {
			t.Fatalf("body %s was accepted", body)
		}
	}
}
