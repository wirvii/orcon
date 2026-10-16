// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"testing"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func parseItem(t *testing.T, raw string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestMaskPathsReplacesMarkedValues: each path of the ADR-0017 form masks what it names, wildcards
// match every element or member, and a path that names nothing masks nothing.
func TestMaskPathsReplacesMarkedValues(t *testing.T) {
	cases := []struct {
		name  string
		item  string
		paths []string
		want  string
	}{
		{"a member", `{"a":1,"b":2}`, []string{"/a"}, `{"a":"[REDACTED]","b":2}`},
		{"a nested member", `{"a":{"b":{"c":"x"},"d":1}}`, []string{"/a/b/c"}, `{"a":{"b":{"c":"[REDACTED]"},"d":1}}`},
		{"a whole object value", `{"a":{"b":1}}`, []string{"/a"}, `{"a":"[REDACTED]"}`},
		{"an array index", `{"a":["x","y"]}`, []string{"/a/1"}, `{"a":["x","[REDACTED]"]}`},
		{"every array element", `{"rows":[{"s":1,"k":2},{"s":3}]}`, []string{"/rows/*/s"}, `{"rows":[{"s":"[REDACTED]","k":2},{"s":"[REDACTED]"}]}`},
		{"every member", `{"a":{"x":1,"y":2}}`, []string{"/a/*"}, `{"a":{"x":"[REDACTED]","y":"[REDACTED]"}}`},
		{"escaped segments", `{"a/b":1,"c~d":2}`, []string{"/a~1b", "/c~0d"}, `{"a/b":"[REDACTED]","c~d":"[REDACTED]"}`},
		{"a path that does not exist", `{"a":1}`, []string{"/b", "/a/c", "/x/*/y"}, `{"a":1}`},
		{"an index past the end", `{"a":[1]}`, []string{"/a/5"}, `{"a":[1]}`},
		{"a malformed path", `{"a":1}`, []string{"a", ""}, `{"a":1}`},
		{"no marks", `{"a":1}`, nil, `{"a":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := parseItem(t, tc.item)
			got := MaskPaths(item, tc.paths)
			want := parseItem(t, tc.want)
			if !jsonvalue.Equal(got, want) {
				gotText, _ := jsonvalue.Marshal(got)
				t.Fatalf("masked item = %s, want %s", gotText, tc.want)
			}
			if !jsonvalue.Equal(item, parseItem(t, tc.item)) {
				t.Fatalf("the item itself changed")
			}
		})
	}
}
