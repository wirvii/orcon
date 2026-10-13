// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import (
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// sectionSeparator splits the request of the sandbox entry point into its
// header and its data sections. JSON text never contains a raw control
// character, so the separator cannot occur inside a section.
const sectionSeparator = "\x01"

// needs says which parts of the context a piece of code reads, so the request
// carries only those. Parsing JSON inside the sandbox is the largest cost of a
// call, and most expressions touch a few properties of a large item.
type needs struct {
	// all asks for every section: the code could not be analyzed or builds
	// names at run time.
	all bool
	// json, binary, vars and params ask for the sections of $json, $binary,
	// $vars and $parameter ($input needs all of json, binary and params).
	json, binary, vars, params bool
	// info asks for the execution, workflow and previous-node headers.
	info bool
	// paths limits $json to these property paths; nil means all of it.
	paths [][]string
}

// needsOf analyzes code and returns what it reads.
func needsOf(code string, noProjection bool) needs {
	a := Analyze(code)
	n := needs{all: a.Syntax() || a.Dynamic}
	input := a.Uses("$input")
	n.json = n.all || a.Uses("$json") || input
	n.binary = n.all || a.Uses("$binary") || input
	n.vars = n.all || a.Uses("$vars")
	n.params = n.all || a.Uses("$parameter") || input
	n.info = n.all || a.Uses("$execution") || a.Uses("$workflow") || a.Uses("$prevNode")
	if n.json && !input && !a.JSONWhole && !noProjection {
		n.paths = a.JSONPaths
	}
	return n
}

// buildRequest writes the request of the prelude entry point (see
// prelude/runtime.js): a JSON header, then the sections $json, $binary,
// $vars and $parameter as JSON text, each preceded by a separator and empty
// when the code does not read it.
func buildRequest(code, mode string, lenient bool, env *Env, n needs) []byte {
	cur := env.current()
	var b strings.Builder
	b.WriteString(`{"e":`)
	b.WriteString(quote(code))
	b.WriteString(`,"m":"`)
	b.WriteString(mode)
	b.WriteString(`","i":`)
	b.WriteString(strconv.Itoa(env.ItemIndex))
	b.WriteString(`,"r":`)
	b.WriteString(strconv.Itoa(env.RunIndex))
	b.WriteString(`,"n":`)
	b.WriteString(strconv.FormatInt(env.Activation.Time().UnixMilli(), 10))
	b.WriteString(`,"z":`)
	b.WriteString(quote(env.Timezone))
	if n.info {
		b.WriteString(`,"x":{"id":`)
		b.WriteString(quote(env.Execution.ID))
		b.WriteString(`,"mode":`)
		b.WriteString(quote(orDefault(env.Execution.Mode, "production")))
		b.WriteString(`,"resumeUrl":`)
		b.WriteString(quote(env.Execution.ResumeURL))
		b.WriteString(`,"resumeFormUrl":`)
		b.WriteString(quote(env.Execution.ResumeFormURL))
		b.WriteString(`},"w":{"id":`)
		b.WriteString(quote(env.Workflow.ID))
		b.WriteString(`,"name":`)
		b.WriteString(quote(env.Workflow.Name))
		b.WriteString(`,"active":`)
		b.WriteString(strconv.FormatBool(env.Workflow.Active))
		b.WriteString(`},"p":{"name":`)
		b.WriteString(quote(env.PrevNode.Name))
		b.WriteString(`,"outputIndex":`)
		b.WriteString(strconv.Itoa(env.PrevNode.OutputIndex))
		b.WriteString(`,"runIndex":`)
		b.WriteString(strconv.Itoa(env.PrevNode.RunIndex))
		b.WriteByte('}')
	}
	if lenient {
		b.WriteString(`,"l":true`)
	}
	b.WriteByte('}')

	section := func(want bool, v jsonvalue.Value) {
		b.WriteString(sectionSeparator)
		if want {
			b.WriteString(jsonText(v))
		}
	}
	item := cur.JSON
	if n.json && n.paths != nil && !item.IsZero() {
		item = project(item, n.paths)
	}
	section(n.json, item)
	section(n.binary, cur.Binary)
	section(n.vars, env.Vars)
	section(n.params, env.Parameters)
	return []byte(b.String())
}

// trie is the set of property paths read from an item.
type trie struct {
	leaf bool
	kids map[string]*trie
}

func (t *trie) add(path []string) {
	if len(path) == 0 {
		t.leaf = true
		return
	}
	if t.kids == nil {
		t.kids = map[string]*trie{}
	}
	k := t.kids[path[0]]
	if k == nil {
		k = &trie{}
		t.kids[path[0]] = k
	}
	k.add(path[1:])
}

// apply keeps of v only what the paths read. A value that is not an object
// where the paths go deeper (a string read for its length, an array read for
// its items) is kept whole, and so is everything below a path that ends.
func (t *trie) apply(v jsonvalue.Value) jsonvalue.Value {
	if t.leaf || len(t.kids) == 0 || v.Kind() != jsonvalue.KindObject {
		return v
	}
	var kept []jsonvalue.Member
	for _, m := range v.Members() {
		if k, ok := t.kids[m.Key]; ok {
			kept = append(kept, jsonvalue.Member{Key: m.Key, Value: k.apply(m.Value)})
		}
	}
	out, err := jsonvalue.Object(kept...)
	if err != nil {
		return v // cannot happen: the keys came from an object
	}
	return out
}

// project returns the part of item that the property paths read.
func project(item jsonvalue.Value, paths [][]string) jsonvalue.Value {
	root := &trie{}
	for _, p := range paths {
		root.add(p)
	}
	if len(paths) == 0 {
		// The code mentions $json only through a chain with no static path, so
		// it reads nothing by name; the item is still given whole to be safe.
		return item
	}
	return root.apply(item)
}
