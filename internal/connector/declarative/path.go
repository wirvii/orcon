// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// step is one segment of a path: an object member, an array index or, for the
// decoding of trigger events only, every item of an array ("[*]").
type step struct {
	key      string
	index    int
	isIndex  bool
	wildcard bool
}

// parsePath reads the JSONPath subset of ADR-0030 §5: "$", ".name", "[n]" and
// "[*]". Filters are not part of this cut and are refused.
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
			if rest[1:end] == "*" {
				steps = append(steps, step{wildcard: true})
				rest = rest[end+1:]
				continue
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

// checkPlainPath reports whether path is a valid path of the subset that reads
// one value: it must parse and must not contain a wildcard.
func checkPlainPath(path string) error {
	steps, err := parsePath(path)
	if err != nil {
		return err
	}
	for _, s := range steps {
		if s.wildcard {
			return fmt.Errorf("%w: path %q has a wildcard where one value is needed", ErrTemplate, path)
		}
	}
	return nil
}

// selectAll returns every node that path reaches, expanding each "[*]" over the
// items of an array. A missing member reaches nothing.
func selectAll(v jsonvalue.Value, path string) ([]jsonvalue.Value, error) {
	steps, err := parsePath(path)
	if err != nil {
		return nil, err
	}
	nodes := []jsonvalue.Value{v}
	for _, s := range steps {
		var next []jsonvalue.Value
		for _, n := range nodes {
			switch {
			case s.wildcard:
				next = append(next, n.Items()...)
			case s.isIndex:
				if c, ok := n.At(s.index); ok {
					next = append(next, c)
				}
			default:
				if c, ok := n.Get(s.key); ok {
					next = append(next, c)
				}
			}
		}
		nodes = next
	}
	return nodes, nil
}

// lookup returns the value at path, and false when the path does not exist. A
// wildcard is refused here: only [selectAll] expands one.
func lookup(v jsonvalue.Value, path string) (jsonvalue.Value, bool, error) {
	steps, err := parsePath(path)
	if err != nil {
		return jsonvalue.Value{}, false, err
	}
	cur := v
	for _, s := range steps {
		if s.wildcard {
			return jsonvalue.Value{}, false, fmt.Errorf("%w: path %q has a wildcard where one value is needed", ErrTemplate, path)
		}
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
