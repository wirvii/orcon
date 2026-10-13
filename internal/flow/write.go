// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Value returns the orcon/flow@1 JSON form of the document. Zero values are
// omitted, so reading a valid document and calling Value gives back the same
// JSON data. The result shares no memory with the document.
func (d *Document) Value() jsonvalue.Value {
	var b builder
	b.str("format", d.Format)
	b.str("id", string(d.ID))
	b.str("name", d.Name)
	b.str("description", d.Description)
	b.val("settings", d.Settings.value())
	b.val("variables", listValue(d.Variables, (*Variable).value))
	b.val("nodes", listValue(d.Nodes, (*Node).value))
	b.val("edges", listValue(d.Edges, (*Edge).value))
	b.val("editor", orEmptyObject(d.Editor))
	b.val("ext", d.Ext)
	return b.build()
}

// builder accumulates the members of a JSON object, skipping absent ones.
type builder struct {
	members []jsonvalue.Member
}

func (b *builder) val(key string, v jsonvalue.Value) {
	if v.IsZero() {
		return
	}
	b.members = append(b.members, jsonvalue.Member{Key: key, Value: v})
}

func (b *builder) str(key, s string) {
	if s != "" {
		b.val(key, jsonvalue.String(s))
	}
}

func (b *builder) integer(key string, n int) {
	if n != 0 {
		b.val(key, jsonvalue.Int(int64(n)))
	}
}

func (b *builder) flag(key string, p *bool) {
	if p != nil {
		b.val(key, jsonvalue.Bool(*p))
	}
}

func (b *builder) strs(key string, items []string) {
	if items == nil {
		return
	}
	vs := make([]jsonvalue.Value, len(items))
	for i, s := range items {
		vs[i] = jsonvalue.String(s)
	}
	b.val(key, jsonvalue.Array(vs...))
}

// build cannot fail: the builder is used with distinct literal keys.
func (b *builder) build() jsonvalue.Value {
	v, err := jsonvalue.Object(b.members...)
	if err != nil {
		panic("flow: duplicate key in a document builder: " + err.Error())
	}
	return v
}

func orEmptyObject(v jsonvalue.Value) jsonvalue.Value {
	if v.IsZero() {
		return (&builder{}).build()
	}
	return v
}

func listValue[T any](items []T, fn func(*T) jsonvalue.Value) jsonvalue.Value {
	vs := make([]jsonvalue.Value, len(items))
	for i := range items {
		vs[i] = fn(&items[i])
	}
	return jsonvalue.Array(vs...)
}

func (s Settings) value() jsonvalue.Value {
	var b builder
	b.str("branchMode", string(s.BranchMode))
	b.str("priority", string(s.Priority))
	b.integer("concurrency", s.Concurrency)
	if s.Timeouts != nil {
		var t builder
		t.str("executionActive", s.Timeouts.ExecutionActive)
		b.val("timeouts", t.build())
	}
	b.str("retention", string(s.Retention))
	if s.MemoryLimit != 0 {
		b.val("memoryLimit", jsonvalue.Int(s.MemoryLimit))
	}
	b.str("errorFlow", string(s.ErrorFlow))
	b.integer("callDepthLimit", s.CallDepthLimit)
	if s.ImportedFrom != nil {
		var i builder
		i.str("sourceVersion", s.ImportedFrom.SourceVersion)
		i.str("reportId", s.ImportedFrom.ReportID)
		b.val("importedFrom", i.build())
	}
	return b.build()
}

func (v Variable) value() jsonvalue.Value {
	var b builder
	b.str("name", v.Name)
	b.str("scope", string(v.Scope))
	b.str("accumulator", string(v.Accumulator))
	b.flag("carry", v.Carry)
	return b.build()
}

func (e Edge) value() jsonvalue.Value {
	var b builder
	b.str("id", string(e.ID))
	var from builder
	from.str("node", string(e.From.Node))
	from.str("port", e.From.Port)
	b.val("from", from.build())
	var to builder
	to.str("node", string(e.To))
	b.val("to", to.build())
	b.str("condition", e.Condition)
	b.flag("default", e.Default)
	b.str("kind", string(e.Kind))
	if e.Order != nil {
		b.val("order", jsonvalue.Int(int64(*e.Order)))
	}
	return b.build()
}

func (n *Node) value() jsonvalue.Value {
	var b builder
	b.str("id", string(n.ID))
	b.str("name", n.Name)
	b.str("element", string(n.Element))
	b.str("type", n.Type)
	b.val("parameters", orEmptyObject(n.Parameters))
	b.flag("disabled", n.Disabled)
	b.val("ext", n.Ext)
	b.strs("writes", n.Writes)
	b.str("routing", string(n.Routing))
	n.writeBlocks(&b)
	b.str("onError", string(n.OnError))
	if n.Concurrency != nil {
		b.val("concurrency", n.Concurrency.value())
	}
	b.str("attachedTo", string(n.AttachedTo))
	b.flag("cancelActivity", n.CancelActivity)
	if n.EventDefinition != nil {
		b.val("eventDefinition", n.EventDefinition.value())
	}
	return b.build()
}

// writeBlocks adds the behavior blocks of a node in document order.
func (n *Node) writeBlocks(b *builder) {
	if n.Join != nil {
		var j builder
		j.str("mode", string(n.Join.Mode))
		j.str("data", string(n.Join.Data))
		j.str("matchField", n.Join.MatchField)
		b.val("join", j.build())
	}
	if n.Loop != nil {
		var l builder
		l.integer("maxIterations", n.Loop.MaxIterations)
		l.strs("carry", n.Loop.Carry)
		b.val("loop", l.build())
	}
	if n.MultiInstance != nil {
		b.val("multiInstance", n.MultiInstance.value())
	}
	if n.Retry != nil {
		b.val("retry", n.Retry.value())
	}
	if n.Timeouts != nil {
		var t builder
		t.str("attempt", n.Timeouts.Attempt)
		t.str("node", n.Timeouts.Node)
		b.val("timeouts", t.build())
	}
	if n.Compensation != nil {
		var c builder
		c.str("handler", string(n.Compensation.Handler))
		c.str("precondition", n.Compensation.Precondition)
		b.val("compensation", c.build())
	}
	if n.Call != nil {
		b.val("call", n.Call.value())
	}
	if n.Wait != nil {
		b.val("wait", n.Wait.value())
	}
	if n.Effect != nil {
		b.val("effect", n.Effect.value())
	}
	if n.Attention != nil {
		var a builder
		a.str("notify", string(n.Attention.Notify))
		b.val("attention", a.build())
	}
}

func (k IntOrKeyword) value() jsonvalue.Value {
	if k.Keyword != "" {
		return jsonvalue.String(k.Keyword)
	}
	return jsonvalue.Int(int64(k.N))
}

func (m *MultiInstance) value() jsonvalue.Value {
	var b builder
	b.val("sequential", jsonvalue.Bool(m.Sequential))
	if m.Concurrency != nil {
		b.val("concurrency", m.Concurrency.value())
	}
	b.str("inputCollection", m.InputCollection)
	b.val("batchSize", jsonvalue.Int(int64(m.BatchSize)))
	b.str("completionCondition", m.CompletionCondition)
	return b.build()
}

func (r *Retry) value() jsonvalue.Value {
	var b builder
	b.val("maxAttempts", jsonvalue.Int(int64(r.MaxAttempts)))
	b.str("initialInterval", r.InitialInterval)
	if r.Factor != 0 {
		f, err := jsonvalue.Float(r.Factor)
		if err != nil {
			f = jsonvalue.Null() // not a JSON number; the document is invalid anyway
		}
		b.val("factor", f)
	}
	b.str("maxInterval", r.MaxInterval)
	b.flag("respectRetryAfter", r.RespectRetryAfter)
	return b.build()
}

func (c *Call) value() jsonvalue.Value {
	var b builder
	b.str("target", c.Target)
	if c.AllowedTargets != nil {
		targets := make([]string, len(c.AllowedTargets))
		for i, t := range c.AllowedTargets {
			targets[i] = string(t)
		}
		b.strs("allowedTargets", targets)
	}
	b.val("version", c.Version.value())
	b.str("mode", string(c.Mode))
	b.val("perItem", jsonvalue.Bool(c.PerItem))
	if c.IntentionalRecursion != nil {
		var r builder
		r.val("maxDepth", jsonvalue.Int(int64(c.IntentionalRecursion.MaxDepth)))
		b.val("intentionalRecursion", r.build())
	}
	return b.build()
}

func (t *Timer) value() jsonvalue.Value {
	var b builder
	b.str("duration", t.Duration)
	b.str("date", t.Date)
	b.str("cycle", t.Cycle)
	return b.build()
}

func (w *Wait) value() jsonvalue.Value {
	var b builder
	b.str("kind", string(w.Kind))
	if w.Timer != nil {
		b.val("timer", w.Timer.value())
	}
	b.str("timeout", w.Timeout)
	b.val("form", w.Form)
	return b.build()
}

func (e *EventDefinition) value() jsonvalue.Value {
	var b builder
	if e.Timer != nil {
		b.val("timer", e.Timer.value())
	}
	if e.Error != nil {
		var c builder
		c.strs("codes", e.Error.Codes)
		b.val("error", c.build())
	}
	if e.Compensation {
		b.val("compensation", (&builder{}).build())
	}
	return b.build()
}

func (e *Effect) value() jsonvalue.Value {
	var b builder
	b.str("effect", string(e.Kind))
	if e.Idempotency != nil {
		var i builder
		i.str("mode", string(e.Idempotency.Mode))
		i.str("ttl", e.Idempotency.TTL)
		i.str("keyField", e.Idempotency.KeyField)
		b.val("idempotency", i.build())
	}
	if e.Verification != nil {
		var v builder
		v.val("reconcile", jsonvalue.Bool(e.Verification.Reconcile))
		b.val("verification", v.build())
	}
	b.str("onUncertain", string(e.OnUncertain))
	b.flag("retrySafe", e.RetrySafe)
	return b.build()
}
