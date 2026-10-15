// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// The completion trackers of a run are the state that is not a row of a
// token: which nodes completed, how far each join got, which loop instances
// are armed. A durable session persists them as one document of the format
// orcon/engine-state@1 next to the execution, and reads it back when a worker
// opens the execution (ADR-0010 section 2: the trackers persist where tokens
// can reach a join or a loop; this implementation persists them for every node
// so that a step needs no static analysis, and the document is rewritten in the
// transaction of every commit point).
//
// Counters that follow from the live tokens (loop occupancy, branch occupancy,
// the tokens that hold a scope or a split instance) are not stored: opening a
// session derives them from the token rows, so a token that changes state
// outside the engine (a timer waking it) cannot make them drift.

// docWriter builds one JSON object, leaving out members that hold their zero
// value so that documents stay small.
type docWriter struct {
	members []jsonvalue.Member
}

func (w *docWriter) put(key string, v jsonvalue.Value) {
	w.members = append(w.members, jsonvalue.Member{Key: key, Value: v})
}

func (w *docWriter) str(key, s string) {
	if s != "" {
		w.put(key, jsonvalue.String(s))
	}
}

func (w *docWriter) num(key string, n int64) {
	if n != 0 {
		w.put(key, jsonvalue.Int(n))
	}
}

func (w *docWriter) flag(key string, b bool) {
	if b {
		w.put(key, jsonvalue.Bool(true))
	}
}

func (w *docWriter) list(key string, items []jsonvalue.Value) {
	if len(items) > 0 {
		w.put(key, jsonvalue.Array(items...))
	}
}

// value returns the object. Keys are written once each by construction, so a
// duplicate is a bug of this file.
func (w *docWriter) value() jsonvalue.Value {
	v, err := jsonvalue.Object(w.members...)
	if err != nil {
		panic("execution: engine state writes a key twice: " + err.Error())
	}
	return v
}

// docReader reads members of a document, remembering the first error.
type docReader struct {
	err error
}

func (d *docReader) failf(format string, args ...any) {
	if d.err == nil {
		d.err = fmt.Errorf("decode document: "+format, args...)
	}
}

func (d *docReader) object(v jsonvalue.Value, what string) bool {
	if v.Kind() != jsonvalue.KindObject {
		d.failf("%s is %s, want an object", what, v.Kind())
		return false
	}
	return true
}

func (d *docReader) str(v jsonvalue.Value, key string) string {
	m, ok := v.Get(key)
	if !ok {
		return ""
	}
	s, ok := m.Str()
	if !ok {
		d.failf("member %q is %s, want a string", key, m.Kind())
	}
	return s
}

func (d *docReader) num(v jsonvalue.Value, key string) int64 {
	m, ok := v.Get(key)
	if !ok {
		return 0
	}
	n, ok := m.Int64()
	if !ok {
		d.failf("member %q is %s, want an integer", key, m.Kind())
	}
	return n
}

func (d *docReader) flag(v jsonvalue.Value, key string) bool {
	m, ok := v.Get(key)
	if !ok {
		return false
	}
	b, ok := m.Bool()
	if !ok {
		d.failf("member %q is %s, want a boolean", key, m.Kind())
	}
	return b
}

func (d *docReader) list(v jsonvalue.Value, key string) []jsonvalue.Value {
	m, ok := v.Get(key)
	if !ok {
		return nil
	}
	if m.Kind() != jsonvalue.KindArray {
		d.failf("member %q is %s, want an array", key, m.Kind())
		return nil
	}
	return m.Items()
}

// The format tag of the document, stored next to it.
const (
	// EngineStateFormat tags the tracker document of an execution.
	EngineStateFormat = "orcon/engine-state@1"
	// ForkLineageFormat tags the fork lineage document of a token.
	ForkLineageFormat = "orcon/fork-lineage@1"
)

func lineageValues(l []ForkRef) []jsonvalue.Value {
	out := make([]jsonvalue.Value, len(l))
	for i, f := range l {
		var w docWriter
		w.str("x", string(f.SplitNodeExecution))
		w.str("n", string(f.SplitNode))
		w.num("b", int64(f.Branch))
		out[i] = w.value()
	}
	return out
}

func (d *docReader) lineage(v jsonvalue.Value, key string) []ForkRef {
	items := d.list(v, key)
	if len(items) == 0 {
		return nil
	}
	out := make([]ForkRef, len(items))
	for i, it := range items {
		if !d.object(it, "a lineage entry") {
			return nil
		}
		out[i] = ForkRef{SplitNodeExecution: NodeExecutionID(d.str(it, "x")), SplitNode: flow.NodeID(d.str(it, "n")), Branch: int(d.num(it, "b"))}
	}
	return out
}

// EncodeTokenDocument writes the fork lineage document of a token: the list
// of the split branches it descends from, and the two marks of a token that
// the interpreter needs to place it again (a join continuation, and the token
// that starts a loop iteration whose returns were merged).
func EncodeTokenDocument(row *TokenRow) ([]byte, error) {
	var w docWriter
	w.list("l", lineageValues(row.Lineage))
	w.flag("p", row.Passed)
	w.flag("m", row.Merged)
	out, err := jsonvalue.Marshal(w.value())
	if err != nil {
		return nil, fmt.Errorf("encode token document: %w", err)
	}
	return out, nil
}

// DecodeTokenDocument reads the document EncodeTokenDocument writes into row.
func DecodeTokenDocument(data []byte, row *TokenRow) error {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return fmt.Errorf("decode token document: %w", err)
	}
	var d docReader
	if !d.object(v, "the token document") {
		return d.err
	}
	row.Lineage = d.lineage(v, "l")
	row.Passed = d.flag(v, "p")
	row.Merged = d.flag(v, "m")
	return d.err
}

func sortedKeys[K comparable](m map[K]struct{}, less func(a, b K) int) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, less)
	return keys
}

func compareNodeKey(r *run) func(a, b nodeKey) int {
	return func(a, b nodeKey) int {
		if c := cmp.Compare(string(r.nodeID(a.node)), string(r.nodeID(b.node))); c != 0 {
			return c
		}
		return cmp.Compare(string(a.scope), string(b.scope))
	}
}

func nodeKeySet[V any](m map[nodeKey]V) map[nodeKey]struct{} {
	out := make(map[nodeKey]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

var errPendingWork = errors.New("engine state: the run still has settling work to do")

// encodeTrackers writes the trackers of the run. A run persists only between
// steps, when everything it had to settle is settled: the one exception is the
// completion signals a new execution queues for its other start events.
func (r *run) encodeTrackers(order []TokenID) ([]byte, error) {
	// A failed execution runs no more; whatever it had left to settle is moot.
	if r.failure == nil && (len(r.dirtyLoops) > 0 || len(r.dirtyGroup) > 0 || len(r.dirtyJoins) > 0 || len(r.dirtyIters) > 0) {
		return nil, errPendingWork
	}
	var root docWriter
	r.encodeNodes(&root)
	r.encodeJoins(&root)
	r.encodeLoops(&root)
	r.encodeEdgeLive(&root)
	r.encodeGroups(&root)
	r.encodeRounds(&root)
	r.encodeReturns(&root)

	if len(r.canceledSplits) > 0 {
		ids := sortedKeys(setOfNE(r.canceledSplits), cmp.Compare)
		vals := make([]jsonvalue.Value, len(ids))
		for i, id := range ids {
			vals[i] = jsonvalue.String(string(id))
		}
		root.list("canceled", vals)
	}
	if len(r.iterations) > 0 {
		scopes := sortedKeys(setOfScope(r.iterations), cmp.Compare)
		vals := make([]jsonvalue.Value, len(scopes))
		for i, s := range scopes {
			vals[i] = jsonvalue.String(string(s))
		}
		root.list("iterations", vals)
	}
	sigs := make([]jsonvalue.Value, len(r.signals))
	for i, sg := range r.signals {
		var w docWriter
		w.str("e", string(r.plan.edges[sg.edge].edge.ID))
		w.str("s", string(sg.scope))
		sigs[i] = w.value()
	}
	root.list("signals", sigs)
	ord := make([]jsonvalue.Value, len(order))
	for i, id := range order {
		ord[i] = jsonvalue.String(string(id))
	}
	root.list("order", ord)
	out, err := jsonvalue.Marshal(root.value())
	if err != nil {
		return nil, fmt.Errorf("encode engine state: %w", err)
	}
	return out, nil
}

func setOfNE(m map[NodeExecutionID]bool) map[NodeExecutionID]struct{} {
	out := make(map[NodeExecutionID]struct{}, len(m))
	for k, v := range m {
		if v {
			out[k] = struct{}{}
		}
	}
	return out
}

func setOfScope(m map[ScopePath]bool) map[ScopePath]struct{} {
	out := make(map[ScopePath]struct{}, len(m))
	for k, v := range m {
		if v {
			out[k] = struct{}{}
		}
	}
	return out
}

func (r *run) encodeNodes(root *docWriter) {
	cmpKey := compareNodeKey(r)
	keys := sortedKeys(nodeKeySet(r.nodes), cmpKey)
	vals := make([]jsonvalue.Value, 0, len(keys))
	for _, k := range keys {
		ns := r.nodes[k]
		var w docWriter
		w.str("n", string(r.nodeID(k.node)))
		w.str("s", string(k.scope))
		w.num("i", int64(ns.inflight))
		w.flag("c", ns.completed)
		sig := make([]jsonvalue.Value, len(ns.signaled))
		for i, b := range ns.signaled {
			sig[i] = jsonvalue.Bool(b)
		}
		w.list("g", sig)
		vals = append(vals, w.value())
	}
	root.list("nodes", vals)
}

func arrivalValues(arr []joinArrival) []jsonvalue.Value {
	out := make([]jsonvalue.Value, len(arr))
	for i, a := range arr {
		var w docWriter
		w.num("slot", int64(a.Slot))
		w.str("t", string(a.Token))
		w.str("p", string(a.Payload))
		w.list("l", lineageValues(a.Lineage))
		out[i] = w.value()
	}
	return out
}

func (r *run) encodeJoins(root *docWriter) {
	keys := sortedKeys(nodeKeySet(r.joins), compareNodeKey(r))
	vals := make([]jsonvalue.Value, 0, len(keys))
	for _, k := range keys {
		js := r.joins[k]
		var w docWriter
		w.str("n", string(r.nodeID(k.node)))
		w.str("s", string(k.scope))
		edges := make([]jsonvalue.Value, len(js.edges))
		for i, e := range js.edges {
			var ew docWriter
			ew.num("s", int64(e.Sent))
			ew.num("a", int64(e.Arrived))
			ew.flag("g", e.Signaled)
			edges[i] = ew.value()
		}
		w.list("e", edges)
		w.list("a", arrivalValues(js.arrivals))
		rounds := make([]jsonvalue.Value, len(js.rounds))
		for i, rd := range js.rounds {
			var rw docWriter
			rw.num("num", int64(rd.number))
			rw.str("x", string(rd.split))
			rw.list("a", arrivalValues(rd.arrivals))
			rw.flag("f", rd.fired)
			rw.flag("r", rd.reset)
			rounds[i] = rw.value()
		}
		w.list("r", rounds)
		w.flag("f", js.fired)
		w.flag("x", js.failed)
		w.flag("y", js.empty)
		vals = append(vals, w.value())
	}
	root.list("joins", vals)
}

func (r *run) encodeLoops(root *docWriter) {
	keys := sortedKeys(nodeKeySet(r.loops), compareNodeKey(r))
	vals := make([]jsonvalue.Value, 0, len(keys))
	for _, k := range keys {
		li := r.loops[k]
		if !li.armed && !li.closed {
			continue
		}
		var w docWriter
		w.str("n", string(r.nodeID(k.node)))
		w.str("s", string(k.scope))
		w.flag("a", li.armed)
		w.flag("c", li.closed)
		vals = append(vals, w.value())
	}
	root.list("loops", vals)
}

func (r *run) encodeEdgeLive(root *docWriter) {
	type entry struct {
		edge  string
		scope ScopePath
		n     int
	}
	entries := make([]entry, 0, len(r.edgeLive))
	for k, n := range r.edgeLive {
		if n > 0 {
			entries = append(entries, entry{string(r.plan.edges[k.edge].edge.ID), k.scope, n})
		}
	}
	slices.SortFunc(entries, func(a, b entry) int {
		if c := cmp.Compare(a.edge, b.edge); c != 0 {
			return c
		}
		return cmp.Compare(a.scope, b.scope)
	})
	vals := make([]jsonvalue.Value, len(entries))
	for i, e := range entries {
		var w docWriter
		w.str("e", e.edge)
		w.str("s", string(e.scope))
		w.num("n", int64(e.n))
		vals[i] = w.value()
	}
	root.list("live", vals)
}

func (r *run) encodeGroups(root *docWriter) {
	ids := make([]NodeExecutionID, 0, len(r.groups))
	for id, g := range r.groups {
		if g.cursor+1 < len(g.siblings) { // a finished group has nothing left to release
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	vals := make([]jsonvalue.Value, len(ids))
	for i, id := range ids {
		g := r.groups[id]
		var w docWriter
		w.str("x", string(id))
		sib := make([]jsonvalue.Value, len(g.siblings))
		for j, t := range g.siblings {
			sib[j] = jsonvalue.String(string(t.id))
		}
		w.list("t", sib)
		keys := make([]jsonvalue.Value, len(g.keys))
		for j, k := range g.keys {
			var kw docWriter
			kw.str("x", string(k.split))
			kw.num("b", int64(k.branch))
			keys[j] = kw.value()
		}
		w.list("k", keys)
		w.num("c", int64(g.cursor))
		vals[i] = w.value()
	}
	root.list("groups", vals)
}

func (r *run) encodeRounds(root *docWriter) {
	ids := make([]NodeExecutionID, 0, len(r.roundJoins))
	for id, keys := range r.roundJoins {
		if len(keys) > 0 {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	vals := make([]jsonvalue.Value, len(ids))
	for i, id := range ids {
		keys := r.roundJoins[id]
		joins := make([]jsonvalue.Value, len(keys))
		for j, k := range keys {
			var kw docWriter
			kw.str("n", string(r.nodeID(k.node)))
			kw.str("s", string(k.scope))
			joins[j] = kw.value()
		}
		var w docWriter
		w.str("x", string(id))
		w.list("j", joins)
		vals[i] = w.value()
	}
	root.list("rounds", vals)
}

func (r *run) encodeReturns(root *docWriter) {
	scopes := make([]ScopePath, 0, len(r.returns))
	for s := range r.returns {
		scopes = append(scopes, s)
	}
	slices.Sort(scopes)
	vals := make([]jsonvalue.Value, len(scopes))
	for i, s := range scopes {
		rec := r.returns[s]
		var w docWriter
		w.str("s", string(s))
		w.str("n", string(r.nodeID(rec.loop)))
		arr := make([]jsonvalue.Value, len(rec.arrived))
		for j, a := range rec.arrived {
			var aw docWriter
			aw.str("e", string(r.plan.edges[a.edge].edge.ID))
			aw.str("t", string(a.token))
			aw.str("p", string(a.payload))
			aw.list("l", lineageValues(a.lineage))
			arr[j] = aw.value()
		}
		w.list("a", arr)
		w.flag("d", rec.started)
		vals[i] = w.value()
	}
	root.list("returns", vals)
}

// decodeTrackers reads the document into the run and returns the creation
// order of the live tokens it recorded.
func (r *run) decodeTrackers(data []byte) ([]TokenID, error) {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("decode engine state: %w", err)
	}
	var d docReader
	if !d.object(v, "the document") {
		return nil, d.err
	}
	r.decodeNodes(&d, v)
	r.decodeJoins(&d, v)
	r.decodeLoops(&d, v)
	r.decodeEdgeLive(&d, v)
	r.decodeGroups(&d, v)
	r.decodeRounds(&d, v)
	r.decodeReturns(&d, v)
	for _, it := range d.list(v, "canceled") {
		s, _ := it.Str()
		r.canceledSplits[NodeExecutionID(s)] = true
	}
	for _, it := range d.list(v, "iterations") {
		s, _ := it.Str()
		r.iterations[ScopePath(s)] = true
	}
	for _, it := range d.list(v, "signals") {
		edge, ok := r.plan.edgeIndex[flow.EdgeID(d.str(it, "e"))]
		if !ok {
			d.failf("a pending signal names an edge the flow does not have")
			break
		}
		r.signals = append(r.signals, signal{edgeKey: edgeKey{edge: edge, scope: ScopePath(d.str(it, "s"))}})
	}
	var order []TokenID
	for _, it := range d.list(v, "order") {
		s, _ := it.Str()
		order = append(order, TokenID(s))
	}
	return order, d.err
}

func (r *run) nodeIndex(d *docReader, id string) (int, bool) {
	i, ok := r.plan.index[flow.NodeID(id)]
	if !ok {
		d.failf("node %q is not in the flow", id)
	}
	return i, ok
}

func (r *run) decodeNodes(d *docReader, v jsonvalue.Value) {
	for _, it := range d.list(v, "nodes") {
		i, ok := r.nodeIndex(d, d.str(it, "n"))
		if !ok {
			return
		}
		ns := r.nodeOf(nodeKey{i, ScopePath(d.str(it, "s"))})
		ns.inflight = int(d.num(it, "i"))
		ns.completed = d.flag(it, "c")
		sig := d.list(it, "g")
		if len(sig) != len(ns.signaled) {
			d.failf("node %q records %d incoming edges, the flow has %d", d.str(it, "n"), len(sig), len(ns.signaled))
			return
		}
		for j, b := range sig {
			ns.signaled[j], _ = b.Bool()
		}
	}
}

func (d *docReader) arrivals(v jsonvalue.Value, key string) []joinArrival {
	items := d.list(v, key)
	out := make([]joinArrival, len(items))
	for i, it := range items {
		out[i] = joinArrival{
			Slot: int(d.num(it, "slot")), Token: TokenID(d.str(it, "t")), Payload: PayloadRef(d.str(it, "p")),
			Lineage: d.lineage(it, "l"),
		}
	}
	return out
}

func (r *run) decodeJoins(d *docReader, v jsonvalue.Value) {
	for _, it := range d.list(v, "joins") {
		i, ok := r.nodeIndex(d, d.str(it, "n"))
		if !ok {
			return
		}
		if r.plan.nodes[i].join == nil {
			d.failf("node %q is not a join", d.str(it, "n"))
			return
		}
		js := r.joinOf(nodeKey{i, ScopePath(d.str(it, "s"))})
		edges := d.list(it, "e")
		if len(edges) != len(js.edges) {
			d.failf("join %q records %d incoming edges, the flow has %d", d.str(it, "n"), len(edges), len(js.edges))
			return
		}
		for j, e := range edges {
			js.edges[j] = joinEdge{Sent: int(d.num(e, "s")), Arrived: int(d.num(e, "a")), Signaled: d.flag(e, "g")}
		}
		js.arrivals = d.arrivals(it, "a")
		for _, rv := range d.list(it, "r") {
			js.rounds = append(js.rounds, &joinRound{
				number: int(d.num(rv, "num")), split: NodeExecutionID(d.str(rv, "x")), arrivals: d.arrivals(rv, "a"),
				fired: d.flag(rv, "f"), reset: d.flag(rv, "r"),
			})
		}
		js.fired, js.failed, js.empty = d.flag(it, "f"), d.flag(it, "x"), d.flag(it, "y")
	}
}

func (r *run) decodeLoops(d *docReader, v jsonvalue.Value) {
	for _, it := range d.list(v, "loops") {
		i, ok := r.nodeIndex(d, d.str(it, "n"))
		if !ok {
			return
		}
		li := r.loopOf(nodeKey{i, ScopePath(d.str(it, "s"))})
		li.armed, li.closed = d.flag(it, "a"), d.flag(it, "c")
	}
}

func (r *run) decodeEdgeLive(d *docReader, v jsonvalue.Value) {
	for _, it := range d.list(v, "live") {
		edge, ok := r.plan.edgeIndex[flow.EdgeID(d.str(it, "e"))]
		if !ok {
			d.failf("a live edge count names an edge the flow does not have")
			return
		}
		r.edgeLive[edgeKey{edge: edge, scope: ScopePath(d.str(it, "s"))}] = int(d.num(it, "n"))
	}
}

// pendingGroup is a sequential group whose sibling tokens are bound once the
// live tokens are known.
type pendingGroup struct {
	id       NodeExecutionID
	siblings []TokenID
	keys     []forkKey
	cursor   int
}

func (r *run) decodeGroups(d *docReader, v jsonvalue.Value) {
	for _, it := range d.list(v, "groups") {
		g := pendingGroup{id: NodeExecutionID(d.str(it, "x")), cursor: int(d.num(it, "c"))}
		for _, t := range d.list(it, "t") {
			s, _ := t.Str()
			g.siblings = append(g.siblings, TokenID(s))
		}
		for _, k := range d.list(it, "k") {
			g.keys = append(g.keys, forkKey{split: NodeExecutionID(d.str(k, "x")), branch: int(d.num(k, "b"))})
		}
		if len(g.siblings) != len(g.keys) || g.cursor < 0 || g.cursor >= len(g.siblings) {
			d.failf("sequential group %q is inconsistent", g.id)
			return
		}
		r.pendingGroups = append(r.pendingGroups, g)
	}
}

func (r *run) decodeRounds(d *docReader, v jsonvalue.Value) {
	for _, it := range d.list(v, "rounds") {
		id := NodeExecutionID(d.str(it, "x"))
		for _, j := range d.list(it, "j") {
			i, ok := r.nodeIndex(d, d.str(j, "n"))
			if !ok {
				return
			}
			r.roundJoins[id] = append(r.roundJoins[id], nodeKey{i, ScopePath(d.str(j, "s"))})
		}
	}
}

func (r *run) decodeReturns(d *docReader, v jsonvalue.Value) {
	for _, it := range d.list(v, "returns") {
		i, ok := r.nodeIndex(d, d.str(it, "n"))
		if !ok {
			return
		}
		rec := &loopReturns{loop: i, started: d.flag(it, "d")}
		for _, a := range d.list(it, "a") {
			edge, ok := r.plan.edgeIndex[flow.EdgeID(d.str(a, "e"))]
			if !ok {
				d.failf("a loop return names an edge the flow does not have")
				return
			}
			rec.arrived = append(rec.arrived, loopReturn{
				edge: edge, token: TokenID(d.str(a, "t")), payload: PayloadRef(d.str(a, "p")), lineage: d.lineage(a, "l"),
			})
		}
		r.returns[ScopePath(d.str(it, "s"))] = rec
	}
}
