// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flowjson_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/codec/flowjson"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// conformanceDir holds the conformance flows, which the flow area owns.
var conformanceDir = filepath.Join("..", "..", "..", "flow", "testdata", "conformance")

func conformanceFiles(tb testing.TB) map[string][]byte {
	tb.Helper()
	files, err := filepath.Glob(filepath.Join(conformanceDir, "*.json"))
	if err != nil || len(files) == 0 {
		tb.Fatalf("no conformance flows in %s (%v)", conformanceDir, err)
	}
	out := make(map[string][]byte, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f) //nolint:gosec // G304: the path comes from a glob over the testdata directory
		if err != nil {
			tb.Fatal(err)
		}
		out[strings.TrimSuffix(filepath.Base(f), ".json")] = data
	}
	return out
}

func conformanceSeeds(tb testing.TB) [][]byte {
	tb.Helper()
	var seeds [][]byte
	for _, data := range conformanceFiles(tb) {
		seeds = append(seeds, data)
	}
	return seeds
}

func conformanceValue(t *testing.T, name string) jsonvalue.Value {
	t.Helper()
	data, ok := conformanceFiles(t)[name]
	if !ok {
		t.Fatalf("no conformance flow %q", name)
	}
	v, err := jsonvalue.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func schemaChecked(t *testing.T) *schemaChecker {
	t.Helper()
	return newSchemaChecker(t, flowjson.Schema())
}

// Edit the value at a JSON Pointer and return the changed copy. The function
// fn receives the old value (absent when missing) and returns the new one;
// returning the zero Value deletes the member or element.
func editAt(t *testing.T, root jsonvalue.Value, pointer string, fn func(old jsonvalue.Value) jsonvalue.Value) jsonvalue.Value {
	t.Helper()
	tokens := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	return editTokens(t, root, tokens, fn)
}

func editTokens(t *testing.T, v jsonvalue.Value, tokens []string, fn func(old jsonvalue.Value) jsonvalue.Value) jsonvalue.Value {
	t.Helper()
	key := strings.ReplaceAll(strings.ReplaceAll(tokens[0], "~1", "/"), "~0", "~")
	last := len(tokens) == 1
	switch v.Kind() {
	case jsonvalue.KindObject:
		old, _ := v.Get(key)
		var next jsonvalue.Value
		if last {
			next = fn(old)
		} else {
			next = editTokens(t, old, tokens[1:], fn)
		}
		if next.IsZero() {
			return v.Without(key)
		}
		return v.With(key, next)
	case jsonvalue.KindArray:
		idx, err := strconv.Atoi(key)
		items := v.Items()
		if err != nil || idx < 0 || idx >= len(items) {
			t.Fatalf("pointer token %q is not an index of the array", key)
		}
		var next jsonvalue.Value
		if last {
			next = fn(items[idx])
		} else {
			next = editTokens(t, items[idx], tokens[1:], fn)
		}
		if next.IsZero() {
			items = append(items[:idx], items[idx+1:]...)
		} else {
			items[idx] = next
		}
		return jsonvalue.Array(items...)
	case jsonvalue.KindInvalid, jsonvalue.KindNull, jsonvalue.KindBool, jsonvalue.KindNumber, jsonvalue.KindString:
		t.Fatalf("cannot descend into %s at %q", v.Kind(), key)
	}
	return v
}

func set(t *testing.T, root jsonvalue.Value, pointer string, nv jsonvalue.Value) jsonvalue.Value {
	t.Helper()
	return editAt(t, root, pointer, func(jsonvalue.Value) jsonvalue.Value { return nv })
}

func remove(t *testing.T, root jsonvalue.Value, pointer string) jsonvalue.Value {
	t.Helper()
	return editAt(t, root, pointer, func(jsonvalue.Value) jsonvalue.Value { return jsonvalue.Value{} })
}

func parse(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(text))
	if err != nil {
		t.Fatalf("parse %q: %v", text, err)
	}
	return v
}

func str(s string) jsonvalue.Value { return jsonvalue.String(s) }
