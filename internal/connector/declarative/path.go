// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// step is one segment of a path: an object member or an array index.
type step struct {
	key     string
	index   int
	isIndex bool
}

// parsePath reads the JSONPath subset of ADR-0030 §5: "$", ".name" and "[n]".
// Filters and wildcards are not part of this cut and are refused.
func parsePath(path string) ([]step, error) {
	if !strings.HasPrefix(path, "$") {
		return nil, fmt.Errorf("%w: path %q must start with $", ErrTemplate, path)
	}
	rest := path[1:]
	var steps []step
	for rest != "" {
		switch rest[0] {
		case '.':
			rest = rest[1:]
			end := strings.IndexAny(rest, ".[")
			if end < 0 {
				end = len(rest)
			}
			name := rest[:end]
			if name == "" || strings.ContainsAny(name, "*?@") {
				return nil, fmt.Errorf("%w: path %q has an unsupported member", ErrTemplate, path)
			}
			steps = append(steps, step{key: name})
			rest = rest[end:]
		case '[':
			end := strings.IndexByte(rest, ']')
			if end < 0 {
				return nil, fmt.Errorf("%w: path %q is not closed", ErrTemplate, path)
			}
			n, err := strconv.Atoi(rest[1:end])
			if err != nil || n < 0 {
				return nil, fmt.Errorf("%w: path %q has an unsupported index", ErrTemplate, path)
			}
			steps = append(steps, step{index: n, isIndex: true})
			rest = rest[end+1:]
		default:
			return nil, fmt.Errorf("%w: path %q is malformed", ErrTemplate, path)
		}
	}
	return steps, nil
}

// lookup returns the value at path, and false when the path does not exist.
func lookup(v jsonvalue.Value, path string) (jsonvalue.Value, bool, error) {
	steps, err := parsePath(path)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}
	cur := v
	for _, s := range steps {
		var ok bool
		if s.isIndex {
			cur, ok = cur.At(s.index)
		} else {
			cur, ok = cur.Get(s.key)
		}
		if !ok {
			return jsonvalue.Value{}, false, nil
		}
	}
	return cur, true, nil
}
