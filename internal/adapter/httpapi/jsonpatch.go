// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
)

// errPatch is the base of the failures of a JSON Patch: an operation that cannot be applied.
var errPatch = errors.New("the patch cannot be applied")

// applyJSONPatch applies the operations of a JSON Patch (RFC 6902) to a JSON document and returns
// the patched document. The patch is all or nothing: the document is decoded into a copy, so the
// first operation that fails leaves nothing applied. A failing operation answers a field error
// that points at the operation in the request body.
func applyJSONPatch(document []byte, patch apigen.JsonPatch) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(document))
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("read the stored draft: %w", err)
	}
	for i := range patch {
		next, err := applyPatchOp(root, &patch[i])
		if err != nil {
			return nil, &documentIssuesError{fields: []problem.FieldError{{
				Pointer: "/" + strconv.Itoa(i), Code: "patch-failed", Message: err.Error(),
			}}}
		}
		root = next
	}
	out, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("encode the patched draft: %w", err)
	}
	return out, nil
}

// applyPatchOp applies one operation and returns the new root.
func applyPatchOp(root any, op *apigen.JsonPatchOperation) (any, error) {
	path, err := parsePointer(op.Path)
	if err != nil {
		return nil, fmt.Errorf("path: %w", err)
	}
	switch op.Op {
	case apigen.Add:
		value, err := patchValue(op)
		if err != nil {
			return nil, err
		}
		return addAt(root, path, value)
	case apigen.Remove:
		next, _, err := removeAt(root, path)
		return next, err
	case apigen.Replace:
		return replaceAt(root, path, op)
	case apigen.Move:
		return moveAt(root, path, op)
	case apigen.Copy:
		return copyAt(root, path, op)
	case apigen.Test:
		return root, testAt(root, path, op)
	default:
		return nil, fmt.Errorf("%w: unknown operation %q", errPatch, op.Op)
	}
}

// replaceAt implements the replace operation: the target must exist.
func replaceAt(root any, path []string, op *apigen.JsonPatchOperation) (any, error) {
	value, err := patchValue(op)
	if err != nil {
		return nil, err
	}
	if _, err := getAt(root, path); err != nil {
		return nil, err
	}
	if len(path) == 0 {
		return value, nil
	}
	next, _, err := removeAt(root, path)
	if err != nil {
		return nil, err
	}
	return addAt(next, path, value)
}

// moveAt implements the move operation: remove at from, then add at path.
func moveAt(root any, path []string, op *apigen.JsonPatchOperation) (any, error) {
	from, err := fromPointer(op)
	if err != nil {
		return nil, err
	}
	if len(from) < len(path) && hasPrefix(path, from) {
		return nil, fmt.Errorf("%w: a value cannot be moved into itself", errPatch)
	}
	next, moved, err := removeAt(root, from)
	if err != nil {
		return nil, err
	}
	return addAt(next, path, moved)
}

// copyAt implements the copy operation: add at path a copy of the value at from.
func copyAt(root any, path []string, op *apigen.JsonPatchOperation) (any, error) {
	from, err := fromPointer(op)
	if err != nil {
		return nil, err
	}
	value, err := getAt(root, from)
	if err != nil {
		return nil, err
	}
	copied, err := cloneJSON(value)
	if err != nil {
		return nil, err
	}
	return addAt(root, path, copied)
}

// testAt implements the test operation: the value at path must equal the value of the operation.
func testAt(root any, path []string, op *apigen.JsonPatchOperation) error {
	value, err := patchValue(op)
	if err != nil {
		return err
	}
	have, err := getAt(root, path)
	if err != nil {
		return err
	}
	if !jsonEqual(have, value) {
		return fmt.Errorf("%w: the value at %s is not the one the test expects", errPatch, op.Path)
	}
	return nil
}

// patchValue is the value of an operation in the form of the document: numbers keep their text.
func patchValue(op *apigen.JsonPatchOperation) (any, error) {
	raw, err := json.Marshal(op.Value)
	if err != nil {
		return nil, fmt.Errorf("%w: the value cannot be read", errPatch)
	}
	return cloneJSON(json.RawMessage(raw))
}

func cloneJSON(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: the value cannot be copied", errPatch)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("%w: the value cannot be copied", errPatch)
	}
	return out, nil
}

func fromPointer(op *apigen.JsonPatchOperation) ([]string, error) {
	if op.From == nil {
		return nil, fmt.Errorf("%w: from is required for %s", errPatch, op.Op)
	}
	from, err := parsePointer(*op.From)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	return from, nil
}

func hasPrefix(path, prefix []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}

// parsePointer reads a JSON Pointer (RFC 6901) into its reference parts.
func parsePointer(p string) ([]string, error) {
	if p == "" {
		return nil, nil
	}
	if !strings.HasPrefix(p, "/") {
		return nil, fmt.Errorf("%w: %q is not a JSON Pointer", errPatch, clip(p, 64))
	}
	parts := strings.Split(p[1:], "/")
	for i, t := range parts {
		t = strings.ReplaceAll(t, "~1", "/")
		parts[i] = strings.ReplaceAll(t, "~0", "~")
	}
	return parts, nil
}

// arrayIndex reads a ref as an index of an array of length n; allowEnd accepts n and "-".
func arrayIndex(ref string, n int, allowEnd bool) (int, error) {
	if allowEnd && ref == "-" {
		return n, nil
	}
	if ref == "" || (len(ref) > 1 && ref[0] == '0') || strings.HasPrefix(ref, "+") || strings.HasPrefix(ref, "-") {
		return 0, fmt.Errorf("%w: %q is not an array index", errPatch, clip(ref, 32))
	}
	i, err := strconv.Atoi(ref)
	if err != nil || i < 0 || i > n || (!allowEnd && i == n) {
		return 0, fmt.Errorf("%w: the index %q is out of range", errPatch, clip(ref, 32))
	}
	return i, nil
}

// getAt returns the value at path.
func getAt(root any, path []string) (any, error) {
	cur := root
	for _, ref := range path {
		switch node := cur.(type) {
		case map[string]any:
			next, ok := node[ref]
			if !ok {
				return nil, fmt.Errorf("%w: there is no member %q", errPatch, clip(ref, 64))
			}
			cur = next
		case []any:
			i, err := arrayIndex(ref, len(node), false)
			if err != nil {
				return nil, err
			}
			cur = node[i]
		default:
			return nil, fmt.Errorf("%w: %q is below a value that has no members", errPatch, clip(ref, 64))
		}
	}
	return cur, nil
}

// addAt inserts value at path and returns the new root (RFC 6902 section 4.1).
func addAt(root any, path []string, value any) (any, error) {
	if len(path) == 0 {
		return value, nil
	}
	parent, err := getAt(root, path[:len(path)-1])
	if err != nil {
		return nil, err
	}
	last := path[len(path)-1]
	switch node := parent.(type) {
	case map[string]any:
		node[last] = value
	case []any:
		i, err := arrayIndex(last, len(node), true)
		if err != nil {
			return nil, err
		}
		return setAt(root, path[:len(path)-1], slices.Insert(node, i, value))
	default:
		return nil, fmt.Errorf("%w: %q is below a value that has no members", errPatch, clip(last, 64))
	}
	return root, nil
}

// removeAt deletes the value at path and returns the new root and the removed value.
func removeAt(root any, path []string) (next, removed any, err error) {
	if len(path) == 0 {
		return nil, root, nil
	}
	parent, err := getAt(root, path[:len(path)-1])
	if err != nil {
		return nil, nil, err
	}
	last := path[len(path)-1]
	switch node := parent.(type) {
	case map[string]any:
		old, ok := node[last]
		if !ok {
			return nil, nil, fmt.Errorf("%w: there is no member %q", errPatch, clip(last, 64))
		}
		delete(node, last)
		return root, old, nil
	case []any:
		i, err := arrayIndex(last, len(node), false)
		if err != nil {
			return nil, nil, err
		}
		old := node[i]
		out, err := setAt(root, path[:len(path)-1], slices.Delete(node, i, i+1))
		return out, old, err
	default:
		return nil, nil, fmt.Errorf("%w: %q is below a value that has no members", errPatch, clip(last, 64))
	}
}

// setAt replaces the value at path (which must exist) with value and returns the new root. It is
// how an array that changed length is put back in its parent, since a slice header is a value.
func setAt(root any, path []string, value any) (any, error) {
	if len(path) == 0 {
		return value, nil
	}
	parent, err := getAt(root, path[:len(path)-1])
	if err != nil {
		return nil, err
	}
	last := path[len(path)-1]
	switch node := parent.(type) {
	case map[string]any:
		node[last] = value
	case []any:
		i, err := arrayIndex(last, len(node), false)
		if err != nil {
			return nil, err
		}
		node[i] = value
	default:
		return nil, fmt.Errorf("%w: %q is below a value that has no members", errPatch, clip(last, 64))
	}
	return root, nil
}

// jsonEqual compares two decoded JSON values: numbers by value, objects without regard to order.
func jsonEqual(a, b any) bool {
	switch x := a.(type) {
	case json.Number:
		y, ok := b.(json.Number)
		if !ok {
			return false
		}
		rx, okx := new(big.Rat).SetString(x.String())
		ry, oky := new(big.Rat).SetString(y.String())
		return okx && oky && rx.Cmp(ry) == 0
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			w, present := y[k]
			if !present || !jsonEqual(v, w) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !jsonEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

// clip shortens text quoted in a message, so a long value sent by a client is not echoed whole.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
