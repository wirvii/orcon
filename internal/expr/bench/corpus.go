// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bench

import (
	"bufio"
	"fmt"
	"io"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Entry is one expression of the corpus: a parameter value that starts with
// "=" found in a node of a public template.
type Entry struct {
	// Template identifies the template in the library.
	Template int
	// Views is the number of views of the template, the measure of usage that
	// weights the entry.
	Views int
	// Node is the name of the node that holds the expression, and NodeType its
	// type; Path is the parameter path inside the node.
	Node, NodeType, Path string
	// Expression is the parameter value, including the leading "=".
	Expression string
}

// Where renders the location of the entry the way an import report names it.
func (e Entry) Where() string {
	return fmt.Sprintf("template %d, node %q (%s), field %s", e.Template, e.Node, e.NodeType, e.Path)
}

// ReadCorpus reads the expressions.jsonl produced by tools/n8ncorpus.
func ReadCorpus(r io.Reader) ([]Entry, error) {
	var out []Entry
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for line := 1; sc.Scan(); line++ {
		v, err := jsonvalue.Parse(sc.Bytes())
		if err != nil {
			return nil, fmt.Errorf("bench: corpus line %d: %w", line, err)
		}
		e, err := entryOf(v)
		if err != nil {
			return nil, fmt.Errorf("bench: corpus line %d: %w", line, err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("bench: read the corpus: %w", err)
	}
	return out, nil
}

func entryOf(v jsonvalue.Value) (Entry, error) {
	text := func(key string) (string, error) {
		m, ok := v.Get(key)
		if !ok {
			return "", fmt.Errorf("missing %q", key)
		}
		s, ok := m.Str()
		if !ok {
			return "", fmt.Errorf("%q is not a string", key)
		}
		return s, nil
	}
	number := func(key string) (int, error) {
		m, ok := v.Get(key)
		if !ok {
			return 0, fmt.Errorf("missing %q", key)
		}
		n, ok := m.Int64()
		if !ok {
			return 0, fmt.Errorf("%q is not an integer", key)
		}
		return int(n), nil
	}
	var (
		e   Entry
		err error
	)
	if e.Template, err = number("template"); err != nil {
		return e, err
	}
	if e.Views, err = number("views"); err != nil {
		return e, err
	}
	if e.Node, err = text("node"); err != nil {
		return e, err
	}
	if e.NodeType, err = text("type"); err != nil {
		return e, err
	}
	if e.Path, err = text("path"); err != nil {
		return e, err
	}
	e.Expression, err = text("expression")
	return e, err
}
