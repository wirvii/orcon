// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
)

func op(kind apigen.JsonPatchOperationOp, path string, value any) apigen.JsonPatchOperation {
	return apigen.JsonPatchOperation{Op: kind, Path: path, Value: value}
}

func from(kind apigen.JsonPatchOperationOp, src, path string) apigen.JsonPatchOperation {
	return apigen.JsonPatchOperation{Op: kind, From: &src, Path: path}
}

func TestApplyJSONPatchFollowsRFC6902(t *testing.T) {
	cases := []struct {
		name  string
		doc   string
		patch apigen.JsonPatch
		want  string
	}{
		{"add a member", `{"a":1}`, apigen.JsonPatch{op(apigen.Add, "/b", "x")}, `{"a":1,"b":"x"}`},
		{"add replaces an existing member", `{"a":1}`, apigen.JsonPatch{op(apigen.Add, "/a", 2)}, `{"a":2}`},
		{"add into an array inserts", `{"l":[1,3]}`, apigen.JsonPatch{op(apigen.Add, "/l/1", 2)}, `{"l":[1,2,3]}`},
		{"add at - appends", `{"l":[1]}`, apigen.JsonPatch{op(apigen.Add, "/l/-", 2)}, `{"l":[1,2]}`},
		{"add into a nested array keeps the parent", `{"n":{"l":[{"x":1}]}}`, apigen.JsonPatch{op(apigen.Add, "/n/l/0", map[string]any{"y": 2})}, `{"n":{"l":[{"y":2},{"x":1}]}}`},
		{"remove a member", `{"a":1,"b":2}`, apigen.JsonPatch{{Op: apigen.Remove, Path: "/a"}}, `{"b":2}`},
		{"remove an array element", `{"l":[1,2,3]}`, apigen.JsonPatch{{Op: apigen.Remove, Path: "/l/1"}}, `{"l":[1,3]}`},
		{"replace a member", `{"a":1}`, apigen.JsonPatch{op(apigen.Replace, "/a", "z")}, `{"a":"z"}`},
		{"replace an array element", `{"l":[1,2]}`, apigen.JsonPatch{op(apigen.Replace, "/l/0", 9)}, `{"l":[9,2]}`},
		{"replace the root", `{"a":1}`, apigen.JsonPatch{op(apigen.Replace, "", map[string]any{"b": true})}, `{"b":true}`},
		{"move a member", `{"a":{"x":1},"b":{}}`, apigen.JsonPatch{from(apigen.Move, "/a/x", "/b/x")}, `{"a":{},"b":{"x":1}}`},
		{"move an array element", `{"l":[1,2,3]}`, apigen.JsonPatch{from(apigen.Move, "/l/0", "/l/-")}, `{"l":[2,3,1]}`},
		{"copy a member", `{"a":{"x":1}}`, apigen.JsonPatch{from(apigen.Copy, "/a", "/b")}, `{"a":{"x":1},"b":{"x":1}}`},
		{"test then change", `{"a":1}`, apigen.JsonPatch{op(apigen.Test, "/a", 1), op(apigen.Replace, "/a", 2)}, `{"a":2}`},
		{"test compares numbers by value", `{"a":1.0}`, apigen.JsonPatch{op(apigen.Test, "/a", 1)}, `{"a":1.0}`},
		{"test compares objects without order", `{"a":{"x":1,"y":2}}`, apigen.JsonPatch{op(apigen.Test, "/a", map[string]any{"y": 2, "x": 1})}, `{"a":{"x":1,"y":2}}`},
		{"escaped tokens", `{"a/b":1,"m~n":2}`, apigen.JsonPatch{op(apigen.Replace, "/a~1b", 3), op(apigen.Replace, "/m~0n", 4)}, `{"a/b":3,"m~n":4}`},
		{"large numbers keep their digits", `{"id":12345678901234567890}`, apigen.JsonPatch{op(apigen.Add, "/n", 1)}, `{"id":12345678901234567890,"n":1}`},
		{"an empty patch changes nothing", `{"a":1}`, apigen.JsonPatch{}, `{"a":1}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := applyJSONPatch([]byte(c.doc), c.patch)
			if err != nil {
				t.Fatal(err)
			}
			if !jsonEqual(mustDecode(t, got), mustDecode(t, []byte(c.want))) {
				t.Fatalf("patched = %s, want %s", got, c.want)
			}
		})
	}
}

func mustDecode(t *testing.T, raw []byte) any {
	t.Helper()
	v, err := cloneJSON(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestApplyJSONPatchRefusesWhatCannotBeApplied(t *testing.T) {
	cases := []struct {
		name  string
		doc   string
		patch apigen.JsonPatch
		at    string
	}{
		{"remove a missing member", `{"a":1}`, apigen.JsonPatch{{Op: apigen.Remove, Path: "/b"}}, "/0"},
		{"replace a missing member", `{"a":1}`, apigen.JsonPatch{op(apigen.Replace, "/b", 1)}, "/0"},
		{"add below a missing parent", `{"a":1}`, apigen.JsonPatch{op(apigen.Add, "/x/y", 1)}, "/0"},
		{"add below a scalar", `{"a":1}`, apigen.JsonPatch{op(apigen.Add, "/a/y", 1)}, "/0"},
		{"index out of range", `{"l":[1]}`, apigen.JsonPatch{{Op: apigen.Remove, Path: "/l/1"}}, "/0"},
		{"insert past the end", `{"l":[1]}`, apigen.JsonPatch{op(apigen.Add, "/l/3", 1)}, "/0"},
		{"index with a leading zero", `{"l":[1,2]}`, apigen.JsonPatch{{Op: apigen.Remove, Path: "/l/01"}}, "/0"},
		{"negative index", `{"l":[1,2]}`, apigen.JsonPatch{{Op: apigen.Remove, Path: "/l/-1"}}, "/0"},
		{"not a pointer", `{"a":1}`, apigen.JsonPatch{op(apigen.Add, "a", 1)}, "/0"},
		{"move without from", `{"a":1}`, apigen.JsonPatch{{Op: apigen.Move, Path: "/b"}}, "/0"},
		{"move into itself", `{"a":{"b":1}}`, apigen.JsonPatch{from(apigen.Move, "/a", "/a/c")}, "/0"},
		{"copy from a missing path", `{"a":1}`, apigen.JsonPatch{from(apigen.Copy, "/z", "/b")}, "/0"},
		{"failing test", `{"a":1}`, apigen.JsonPatch{op(apigen.Replace, "/a", 2), op(apigen.Test, "/a", 3)}, "/1"},
		{"test of a missing path", `{"a":1}`, apigen.JsonPatch{op(apigen.Test, "/b", 1)}, "/0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := applyJSONPatch([]byte(c.doc), c.patch)
			var issues *documentIssuesError
			if !errors.As(err, &issues) || len(issues.fields) != 1 || issues.fields[0].Pointer != c.at {
				t.Fatalf("err = %v, want one field error at %s", err, c.at)
			}
		})
	}
}
