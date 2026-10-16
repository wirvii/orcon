// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Redacted is the text that replaces a sensitive value in the history of an execution
// (ADR-0017 section 3).
const Redacted = "[REDACTED]"

// MaskPaths returns the item with the value at each sensitive path replaced by [Redacted]. A
// path is a JSON Pointer (RFC 6901) into the item; a segment that is exactly "*" matches every
// array element and every object member at that level (ADR-0007 section 3). A path that does not
// exist in the item masks nothing and is not an error. The item is not changed.
func MaskPaths(item jsonvalue.Value, paths []string) jsonvalue.Value {
	for _, p := range paths {
		segments, ok := pointerSegments(p)
		if !ok {
			continue
		}
		item = maskAt(item, segments)
	}
	return item
}

// pointerSegments splits a JSON Pointer into its unescaped segments. The empty pointer, which
// names the whole item, and a pointer that does not start with "/" mask nothing.
func pointerSegments(p string) ([]string, bool) {
	rest, ok := strings.CutPrefix(p, "/")
	if !ok {
		return nil, false
	}
	parts := strings.Split(rest, "/")
	for i, s := range parts {
		if s == "*" {
			continue
		}
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
	}
	return parts, true
}

func maskAt(v jsonvalue.Value, segments []string) jsonvalue.Value {
	if len(segments) == 0 {
		return jsonvalue.String(Redacted)
	}
	head, rest := segments[0], segments[1:]
	if v.Kind() == jsonvalue.KindObject {
		return maskObject(v, head, rest)
	}
	if v.Kind() == jsonvalue.KindArray {
		return maskArray(v, head, rest)
	}
	return v
}

func maskObject(v jsonvalue.Value, head string, rest []string) jsonvalue.Value {
	if head == "*" {
		out := v
		for _, m := range v.Members() {
			out = out.With(m.Key, maskAt(m.Value, rest))
		}
		return out
	}
	child, ok := v.Get(head)
	if !ok {
		return v
	}
	return v.With(head, maskAt(child, rest))
}

func maskArray(v jsonvalue.Value, head string, rest []string) jsonvalue.Value {
	items := v.Items()
	changed := false
	for i := range items {
		if head != "*" && head != strconv.Itoa(i) {
			continue
		}
		items[i] = maskAt(items[i], rest)
		changed = true
	}
	if !changed {
		return v
	}
	return jsonvalue.Array(items...)
}
