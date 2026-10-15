// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flowjson_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/codec/flowjson"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func sensitiveArray(paths ...string) jsonvalue.Value {
	vs := make([]jsonvalue.Value, len(paths))
	for i, p := range paths {
		vs[i] = jsonvalue.String(p)
	}
	return jsonvalue.Array(vs...)
}

// TestSensitivePathsSchemaAndReaderAgree is the differential test of the
// sensitivePaths member: the published schema and the Go reader plus level-1
// validation accept and reject exactly the same documents.
func TestSensitivePathsSchemaAndReaderAgree(t *testing.T) {
	t.Parallel()
	many := make([]string, 65)
	for i := range many {
		many[i] = fmt.Sprintf("/f%d", i)
	}
	cases := []struct {
		name  string
		value jsonvalue.Value
		valid bool
	}{
		{"one path", sensitiveArray("/password"), true},
		{"wildcard and escapes", sensitiveArray("/items/*/token", "/a~0b", "/a~1b"), true},
		{"root key with an empty name", sensitiveArray("/"), true},
		{"64 paths", sensitiveArray(many[:64]...), true},
		{"512 code points", sensitiveArray("/" + strings.Repeat("é", 511)), true},
		{"empty array", jsonvalue.Array(), false},
		{"no leading slash", sensitiveArray("password"), false},
		{"empty string", sensitiveArray(""), false},
		{"tilde 2", sensitiveArray("/a~2"), false},
		{"trailing tilde", sensitiveArray("/a~"), false},
		{"513 code points", sensitiveArray("/" + strings.Repeat("é", 512)), false},
		{"65 paths", sensitiveArray(many...), false},
		{"duplicate", sensitiveArray("/a", "/a"), false},
		{"not an array", jsonvalue.String("/a"), false},
		{"element not a string", jsonvalue.Array(jsonvalue.Int(1)), false},
	}
	c := schemaChecked(t)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := set(t, conformanceValue(t, "linear"), "/nodes/1/sensitivePaths", tt.value)
			if got := c.valid(v); got != tt.valid {
				t.Errorf("schema valid = %t, want %t (%v)", got, tt.valid, c.errors(v))
			}
			if got := !goRejectsShape(t, v); got != tt.valid {
				t.Errorf("reader and validator accept = %t, want %t", got, tt.valid)
			}
		})
	}
}

// TestSensitivePathsRoundTrip checks that a document with marks survives
// Decode and Encode byte for byte, in the member order of the format.
func TestSensitivePathsRoundTrip(t *testing.T) {
	t.Parallel()
	v := set(t, conformanceValue(t, "linear"), "/nodes/1/sensitivePaths", sensitiveArray("/z", "/a/*/b", "/m~0n~1o"))
	doc, err := flowjson.Decode(mustMarshal(t, v))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got, err := flowjson.Encode(&doc)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	again, err := flowjson.Decode(got)
	if err != nil {
		t.Fatalf("Decode(Encode(doc)): %v", err)
	}
	second, err := flowjson.Encode(&again)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, second) {
		t.Errorf("Encode is not stable:\n%s\n%s", got, second)
	}
	if !strings.Contains(string(got), `"sensitivePaths":["/z","/a/*/b","/m~0n~1o"]`) {
		t.Errorf("encoded document lost or reordered the marks: %s", got)
	}
	if r := flow.Validate(&doc); len(r.Errors()) > 0 {
		t.Errorf("Validate errors = %v", r.Errors())
	}
}
