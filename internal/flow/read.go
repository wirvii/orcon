// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"fmt"
	"math"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// maxStructureIssues bounds the findings of one read, so hostile input
// cannot make the report grow without limit.
const maxStructureIssues = 100

// Read upgrades a stored document to the current format and maps it to a
// [Document]. The input is never modified. See [Upgrade] and [FromValue].
func Read(v jsonvalue.Value) (Document, error) {
	upgraded, err := Upgrade(v)
	if err != nil {
		return Document{}, err
	}
	return FromValue(upgraded)
}

// FromValue maps an orcon/flow@1 JSON value to a [Document]. It checks the
// shape only: member names, JSON types, required members and non-empty
// strings. Member values such as enumerations, ranges and references are the
// business of [Validate]. A shape problem is returned as a
// [*ValidationError] listing every problem found.
func FromValue(v jsonvalue.Value) (Document, error) {
	d := &decoder{}
	for _, n := range jsonvalue.InexactNumbers(v, maxStructureIssues) {
		d.failf(n.Pointer, "number %s cannot be hashed without loss: integers must lie within ±%d and decimals within double precision (write larger values as strings)", n.Text, jsonvalue.MaxSafeInteger)
	}
	doc := d.document(v)
	if len(d.issues) > 0 {
		return Document{}, &ValidationError{Report: Report{Issues: d.issues}}
	}
	return doc, nil
}

type decoder struct {
	issues []Issue
}

func (d *decoder) failf(path, format string, args ...any) {
	if len(d.issues) == maxStructureIssues {
		d.issues = append(d.issues, Issue{Severity: SeverityError, Code: CodeStructure, Path: "", Message: "too many problems; the rest are not listed"})
		return
	}
	if len(d.issues) > maxStructureIssues {
		return
	}
	d.issues = append(d.issues, Issue{Severity: SeverityError, Code: CodeStructure, Path: path, Message: fmt.Sprintf(format, args...)})
}

// obj reads the members of one JSON object and reports the ones nobody asked for.
type obj struct {
	d    *decoder
	path string
	v    jsonvalue.Value
	seen map[string]struct{}
}

// object opens v as an object, or reports a type error.
func (d *decoder) object(path string, v jsonvalue.Value) (*obj, bool) {
	if v.Kind() != jsonvalue.KindObject {
		d.failf(path, "must be an object, found %s", v.Kind())
		return nil, false
	}
	return &obj{d: d, path: path, v: v, seen: map[string]struct{}{}}, true
}

func (o *obj) at(key string) string { return o.path + "/" + escapePointer(key) }

func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// get returns member key and marks it as read. A missing required member is
// reported here, so callers only handle the value.
func (o *obj) get(key string, required bool) (jsonvalue.Value, bool) {
	o.seen[key] = struct{}{}
	v, ok := o.v.Get(key)
	if !ok && required {
		o.d.failf(o.at(key), "missing required property %q", key)
	}
	return v, ok
}

func (o *obj) str(key string, required bool) string {
	v, ok := o.get(key, required)
	if !ok {
		return ""
	}
	s, isStr := v.Str()
	switch {
	case !isStr:
		o.d.failf(o.at(key), "must be a string, found %s", v.Kind())
	case s == "":
		o.d.failf(o.at(key), "must not be empty; omit the property instead")
	}
	return s
}

// integer reads an integer of at least 1. Zero is refused here, not only in
// validation, because the model cannot tell an explicit 0 from an absent
// member and the document must read back to the same data.
func (o *obj) integer(key string, required bool) int {
	return int(o.integerBetween(key, required, 1, math.MaxInt32))
}

// integer64 reads a byte count: at least 1 and at most 2^53-1, the largest
// integer a JSON number carries exactly.
func (o *obj) integer64(key string) int64 {
	return o.integerBetween(key, false, 1, jsonvalue.MaxSafeInteger)
}

func (o *obj) integerBetween(key string, required bool, lowest, highest int64) int64 {
	v, ok := o.get(key, required)
	if !ok {
		return 0
	}
	n, isInt := v.Int64()
	if !isInt || n < lowest || n > highest {
		o.d.failf(o.at(key), "must be an integer from %d to %d", lowest, highest)
		return 0
	}
	return n
}

func (o *obj) number(key string) float64 {
	v, ok := o.get(key, false)
	if !ok {
		return 0
	}
	f, isNum := v.Float64()
	if !isNum || f == 0 {
		o.d.failf(o.at(key), "must be a number other than zero")
		return 0
	}
	return f
}

// boolean reads a boolean; present tells whether the member exists.
func (o *obj) boolean(key string, required bool) (value, present bool) {
	v, ok := o.get(key, required)
	if !ok {
		return false, false
	}
	b, isBool := v.Bool()
	if !isBool {
		o.d.failf(o.at(key), "must be a boolean, found %s", v.Kind())
		return false, false
	}
	return b, true
}

func (o *obj) optBool(key string) *bool {
	b, ok := o.boolean(key, false)
	if !ok {
		return nil
	}
	return &b
}

// opaque reads a member that must be an object whose content the format
// leaves open (parameters, editor, ext, form).
func (o *obj) opaque(key string, required bool) jsonvalue.Value {
	v, ok := o.get(key, required)
	if !ok {
		return jsonvalue.Value{}
	}
	if v.Kind() != jsonvalue.KindObject {
		o.d.failf(o.at(key), "must be an object, found %s", v.Kind())
		return jsonvalue.Value{}
	}
	return v
}

// child opens a nested object.
func (o *obj) child(key string, required bool) (*obj, bool) {
	v, ok := o.get(key, required)
	if !ok {
		return nil, false
	}
	return o.d.object(o.at(key), v)
}

// array returns the items of an array member together with their paths.
func (o *obj) array(key string, required bool) ([]jsonvalue.Value, bool) {
	v, ok := o.get(key, required)
	if !ok {
		return nil, false
	}
	if v.Kind() != jsonvalue.KindArray {
		o.d.failf(o.at(key), "must be an array, found %s", v.Kind())
		return nil, false
	}
	return v.Items(), true
}

// strings reads an array of non-empty strings. Absent returns nil; present
// returns a non-nil slice even when empty.
func (o *obj) strings(key string) []string {
	items, ok := o.array(key, false)
	if !ok {
		return nil
	}
	if len(items) == 0 {
		o.d.failf(o.at(key), "must not be empty; omit the property instead")
		return nil
	}
	out := make([]string, 0, len(items))
	for i, item := range items {
		s, isStr := item.Str()
		if !isStr || s == "" {
			o.d.failf(fmt.Sprintf("%s/%d", o.at(key), i), "must be a non-empty string")
			continue
		}
		out = append(out, s)
	}
	return out
}

// done reports every member that was not read.
func (o *obj) done() {
	for _, m := range o.v.Members() {
		if _, ok := o.seen[m.Key]; !ok {
			o.d.failf(o.at(m.Key), "unknown property %q", m.Key)
		}
	}
}

func (d *decoder) document(v jsonvalue.Value) Document {
	var doc Document
	o, ok := d.object("", v)
	if !ok {
		return doc
	}
	doc.Format = o.str("format", true)
	doc.ID = ID(o.str("id", true))
	doc.Name = o.str("name", true)
	doc.Description = o.str("description", false)
	if s, ok := o.child("settings", true); ok {
		doc.Settings = d.settings(s)
	}
	doc.Variables = readList(o, "variables", d.variable)
	doc.Nodes = readList(o, "nodes", d.node)
	doc.Edges = readList(o, "edges", d.edge)
	doc.Editor = o.opaque("editor", true)
	doc.Ext = o.opaque("ext", false)
	o.done()
	return doc
}

// readList decodes a required array of objects with fn.
func readList[T any](o *obj, key string, fn func(*obj) T) []T {
	items, ok := o.array(key, true)
	out := make([]T, 0, len(items))
	if !ok {
		return out
	}
	for i, item := range items {
		child, isObj := o.d.object(fmt.Sprintf("%s/%d", o.at(key), i), item)
		if !isObj {
			continue
		}
		out = append(out, fn(child))
	}
	return out
}

func (d *decoder) settings(o *obj) Settings {
	s := Settings{
		BranchMode:     BranchMode(o.str("branchMode", false)),
		Priority:       Priority(o.str("priority", false)),
		Concurrency:    o.integer("concurrency", false),
		Retention:      Retention(o.str("retention", false)),
		MemoryLimit:    o.integer64("memoryLimit"),
		ErrorFlow:      ID(o.str("errorFlow", false)),
		CallDepthLimit: o.integer("callDepthLimit", false),
	}
	if t, ok := o.child("timeouts", false); ok {
		s.Timeouts = &ExecutionTimeouts{ExecutionActive: t.str("executionActive", false)}
		t.done()
	}
	if im, ok := o.child("importedFrom", false); ok {
		s.ImportedFrom = &ImportedFrom{SourceVersion: im.str("sourceVersion", true), ReportID: im.str("reportId", true)}
		im.done()
	}
	o.done()
	return s
}

func (d *decoder) variable(o *obj) Variable {
	v := Variable{
		Name:        o.str("name", true),
		Scope:       VarScope(o.str("scope", true)),
		Accumulator: Accumulator(o.str("accumulator", false)),
		Carry:       o.optBool("carry"),
	}
	o.done()
	return v
}

func (d *decoder) edge(o *obj) Edge {
	e := Edge{
		ID:        EdgeID(o.str("id", true)),
		Condition: o.str("condition", false),
		Default:   o.optBool("default"),
		Kind:      EdgeKind(o.str("kind", true)),
		Order:     int(o.integerBetween("order", false, 1, math.MaxInt32)),
	}
	if from, ok := o.child("from", true); ok {
		e.From = Endpoint{Node: NodeID(from.str("node", true)), Port: from.str("port", true)}
		from.done()
	}
	if to, ok := o.child("to", true); ok {
		e.To = NodeID(to.str("node", true))
		to.done()
	}
	o.done()
	return e
}

func (d *decoder) node(o *obj) Node {
	n := Node{
		ID:         NodeID(o.str("id", true)),
		Name:       o.str("name", true),
		Element:    Element(o.str("element", true)),
		Type:       o.str("type", true),
		Parameters: o.opaque("parameters", true),
		Disabled:   o.optBool("disabled"),
		Ext:        o.opaque("ext", false),
		Routing:    Routing(o.str("routing", false)),
		OnError:    OnError(o.str("onError", false)),
		AttachedTo: NodeID(o.str("attachedTo", false)),
	}
	n.CancelActivity = o.optBool("cancelActivity")
	if c, ok := o.child("connector", false); ok {
		n.Connector = &ConnectorPin{Version: c.str("version", true), Digest: c.str("digest", true)}
		c.done()
	}
	d.nodeBlocks(o, &n)
	o.done()
	return n
}

// nodeBlocks reads the optional behavior blocks of a node.
func (d *decoder) nodeBlocks(o *obj, n *Node) {
	if c, ok := o.child("join", false); ok {
		n.Join = &Join{Mode: JoinMode(c.str("mode", true)), Data: JoinData(c.str("data", true)), MatchField: c.str("matchField", false), Remaining: JoinRemaining(c.str("remaining", false))}
		c.done()
	}
	if c, ok := o.child("loop", false); ok {
		n.Loop = &Loop{MaxIterations: c.integer("maxIterations", true), Carry: c.strings("carry")}
		c.done()
	}
	if c, ok := o.child("multiInstance", false); ok {
		n.MultiInstance = d.multiInstance(c)
	}
	if c, ok := o.child("retry", false); ok {
		n.Retry = &Retry{
			MaxAttempts:       c.integer("maxAttempts", true),
			InitialInterval:   c.str("initialInterval", false),
			Factor:            c.number("factor"),
			MaxInterval:       c.str("maxInterval", false),
			RespectRetryAfter: c.optBool("respectRetryAfter"),
		}
		c.done()
	}
	if c, ok := o.child("timeouts", false); ok {
		n.Timeouts = &NodeTimeouts{Attempt: c.str("attempt", false), Node: c.str("node", false)}
		c.done()
	}
	if _, ok := o.v.Get("concurrency"); ok {
		n.Concurrency = d.intOrKeyword(o, "concurrency")
	}
	if c, ok := o.child("compensation", false); ok {
		n.Compensation = &Compensation{Handler: NodeID(c.str("handler", true)), Precondition: c.str("precondition", false)}
		c.done()
	}
	if c, ok := o.child("call", false); ok {
		n.Call = d.call(c)
	}
	if c, ok := o.child("wait", false); ok {
		n.Wait = d.wait(c)
	}
	if c, ok := o.child("eventDefinition", false); ok {
		n.EventDefinition = d.eventDefinition(c)
	}
	if c, ok := o.child("trigger", false); ok {
		n.Trigger = d.trigger(c)
		c.done()
	}
	if c, ok := o.child("attention", false); ok {
		n.Attention = &Attention{Notify: AttentionNotify(c.str("notify", true))}
		c.done()
	}
}

// intOrKeyword reads a member that is an integer or a string keyword.
func (d *decoder) intOrKeyword(o *obj, key string) *IntOrKeyword {
	v, _ := o.get(key, false)
	if s, ok := v.Str(); ok {
		if s == "" {
			d.failf(o.at(key), "must not be empty")
		}
		return &IntOrKeyword{Keyword: s}
	}
	n, ok := v.Int64()
	if !ok || n < 1 || n > math.MaxInt32 {
		d.failf(o.at(key), "must be an integer from 1 to %d or a keyword string", math.MaxInt32)
		return &IntOrKeyword{}
	}
	return &IntOrKeyword{N: int(n)}
}

func (d *decoder) multiInstance(o *obj) *MultiInstance {
	m := &MultiInstance{
		InputCollection:     o.str("inputCollection", false),
		BatchSize:           o.integer("batchSize", true),
		CompletionCondition: o.str("completionCondition", false),
	}
	m.Sequential, _ = o.boolean("sequential", true)
	if _, ok := o.v.Get("concurrency"); ok {
		m.Concurrency = d.intOrKeyword(o, "concurrency")
	}
	o.done()
	return m
}

func (d *decoder) call(o *obj) *Call {
	c := &Call{
		Target: o.str("target", true),
		Mode:   CallMode(o.str("mode", true)),
	}
	if targets := o.strings("allowedTargets"); targets != nil {
		c.AllowedTargets = make([]ID, len(targets))
		for i, t := range targets {
			c.AllowedTargets[i] = ID(t)
		}
	}
	if _, ok := o.v.Get("version"); ok {
		c.Version = *d.intOrKeyword(o, "version")
	} else {
		o.d.failf(o.at("version"), "missing required property %q", "version")
	}
	c.PerItem, _ = o.boolean("perItem", true)
	if r, ok := o.child("intentionalRecursion", false); ok {
		c.IntentionalRecursion = &IntentionalRecursion{MaxDepth: r.integer("maxDepth", true)}
		r.done()
	}
	o.done()
	return c
}

func (d *decoder) timer(o *obj) *Timer {
	t := &Timer{Duration: o.str("duration", false), Date: o.str("date", false), Cycle: o.str("cycle", false)}
	o.done()
	return t
}

func (d *decoder) wait(o *obj) *Wait {
	w := &Wait{Kind: WaitKind(o.str("kind", true)), Timeout: o.str("timeout", false), Form: o.opaque("form", false)}
	if t, ok := o.child("timer", false); ok {
		w.Timer = d.timer(t)
	}
	o.done()
	return w
}

func (d *decoder) eventDefinition(o *obj) *EventDefinition {
	e := &EventDefinition{}
	if t, ok := o.child("timer", false); ok {
		e.Timer = d.timer(t)
	}
	if c, ok := o.child("error", false); ok {
		e.Error = &ErrorDefinition{Codes: c.strings("codes")}
		c.done()
	}
	if c, ok := o.child("compensation", false); ok {
		e.Compensation = true
		c.done()
	}
	o.done()
	return e
}
