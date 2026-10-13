// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// The pattern sources below are shared with the published JSON Schema, which
// the flowjson tests compare against these constants so the two cannot drift.
// They use only constructs that RE2 and ECMAScript read the same way.
const (
	// IDPattern is the form of node and edge ids.
	IDPattern = `^[A-Za-z0-9_-]{1,64}$`
	// UUIDPattern is the canonical lowercase text form of a UUID, the form of
	// flow ids (ADR-0015 sections 2 and 3).
	UUIDPattern = `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	// InstantPattern is an RFC 3339 date-time in UTC, written with a "Z".
	InstantPattern = `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z$`
	// VariableNamePattern is the form of variable names.
	VariableNamePattern = `^[A-Za-z_][A-Za-z0-9_]{0,63}$`
	// NodeTypePattern is the form of node types: "<namespace>.<name>@<major>".
	NodeTypePattern = `^[a-z][a-z0-9-]*(\.[A-Za-z][A-Za-z0-9_-]*)+@[1-9][0-9]*$`
	// PortPattern is the form of port names.
	PortPattern = `^[A-Za-z][A-Za-z0-9_-]*(:[A-Za-z0-9_.-]+)?$`
	// ExtKeyPattern is the form of extension keys: "vendor.key".
	ExtKeyPattern = `^[a-z0-9][a-z0-9-]*\.[A-Za-z0-9_.-]+$`
	// DurationPattern is an ISO 8601 duration of weeks, days, hours, minutes
	// and seconds with at least one component. Years and months are excluded
	// because their length varies.
	DurationPattern = `^` + durationCore + `$`
	// CyclePattern is a repeating interval: "R[n]/<duration>".
	CyclePattern = `^R\d*/` + durationCore + `$`

	durationCore = `P(\d+W|\d+D(T(\d+H(\d+M)?(\d+(\.\d+)?S)?|\d+M(\d+(\.\d+)?S)?|\d+(\.\d+)?S))?|T(\d+H(\d+M)?(\d+(\.\d+)?S)?|\d+M(\d+(\.\d+)?S)?|\d+(\.\d+)?S))`
)

var (
	idPattern       = regexp.MustCompile(IDPattern)
	uuidPattern     = regexp.MustCompile(UUIDPattern)
	instantPattern  = regexp.MustCompile(InstantPattern)
	variablePattern = regexp.MustCompile(VariableNamePattern)
	typePattern     = regexp.MustCompile(NodeTypePattern)
	portPattern     = regexp.MustCompile(PortPattern)
	extKeyPattern   = regexp.MustCompile(ExtKeyPattern)
	durationPattern = regexp.MustCompile(DurationPattern)
	cyclePattern    = regexp.MustCompile(CyclePattern)
)

const (
	maxFlowNameRunes = 256
	maxNodeNameRunes = 128
	maxPortLength    = 80
)

// Validate checks a document against every static rule of ADR-0007 section 9
// and ADR-0010 section 11. It returns all findings of the first level that has
// errors (structure, then references) so one mistake does not bury the user in
// follow-up noise; once those pass it runs control-flow, data-access, safety
// and call rules together. The document is never modified.
func Validate(d *Document) Report {
	return validate(d, validatorConfig{})
}

// ValidateWithTypes is [Validate] plus the node-type rules of level 5: every
// node type must be known to the catalog, and the safety contract of an
// effect type must be valid (rule V10, ADR-0029 section 5). Types the format
// itself defines (orcon.loop, orcon.if, orcon.switch) are not looked up.
func ValidateWithTypes(ctx context.Context, d *Document, types NodeTypes) Report {
	return validate(d, validatorConfig{lookup: func(nodeType string) (NodeType, bool) {
		return types.Lookup(ctx, nodeType)
	}})
}

// validatorConfig carries what a validation run needs besides the document.
// Both members are optional: no lookup skips the node-type rules, and no
// writers function uses [writersOf].
type validatorConfig struct {
	lookup  func(nodeType string) (NodeType, bool)
	writers func(*Node) []string
}

func validate(d *Document, cfg validatorConfig) Report {
	var c collector
	v := &validator{doc: d, r: &c, cfg: cfg}
	v.structure()
	v.references()
	if c.hasErrors() {
		return c.report()
	}
	g := newGraph(d)
	v.control(g)
	v.data(g)
	v.loopExits(g)
	v.safety()
	v.calls()
	return c.report()
}

type validator struct {
	doc *Document
	r   *collector
	cfg validatorConfig
}

// writers lists the variables a node writes, through the configured function.
func (v *validator) writers(n *Node) []string {
	if v.cfg.writers != nil {
		return v.cfg.writers(n)
	}
	return writersOf(n)
}

// ptr builds a JSON Pointer from path segments.
func ptr(parts ...any) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteByte('/')
		b.WriteString(escapePointer(fmt.Sprint(p)))
	}
	return b.String()
}

// checkEnum reports a value outside its closed set. A value that matches an
// allowed one when case is ignored gets a hint, because lowercase spellings
// are the common mistake (ADR-0007 section 11).
func checkEnum[T ~string](r *collector, path string, v T, allowed []T) {
	if slices.Contains(allowed, v) {
		return
	}
	names := make([]string, len(allowed))
	for i, a := range allowed {
		names[i] = string(a)
	}
	hint := ""
	for _, a := range allowed {
		if strings.EqualFold(string(a), string(v)) {
			hint = fmt.Sprintf("; write it as %q (enumerated values are UPPER_SNAKE)", a)
			break
		}
	}
	r.errorf(CodeEnum, path, "%q is not one of %s%s", v, strings.Join(names, ", "), hint)
}

func checkOptEnum[T ~string](r *collector, path string, v T, allowed []T) {
	if v != "" {
		checkEnum(r, path, v, allowed)
	}
}

func checkPattern(r *collector, path, what, s string, re *regexp.Regexp) {
	if !re.MatchString(s) {
		r.errorf(CodeValue, path, "%s %q is not valid; it must match %s", what, s, re)
	}
}

// checkDuration validates an ISO 8601 duration made of weeks, days, hours,
// minutes and seconds (years and months vary in length and are not allowed).
func checkDuration(r *collector, path, s string) {
	if s == "" {
		return
	}
	if !durationPattern.MatchString(s) {
		r.errorf(CodeValue, path, "%q is not an ISO 8601 duration such as PT30S, PT1H30M or P2D", s)
	}
}

func checkCycle(r *collector, path, s string) {
	if !cyclePattern.MatchString(s) {
		r.errorf(CodeValue, path, "%q is not a repeating interval such as R/PT10M or R5/PT10M", s)
	}
}

func checkExt(r *collector, path string, ext jsonvalue.Value) {
	if ext.IsZero() {
		return
	}
	if ext.Kind() != jsonvalue.KindObject {
		r.errorf(CodeStructure, path, "must be an object, found %s", ext.Kind())
		return
	}
	for _, m := range ext.Members() {
		switch {
		case strings.HasPrefix(m.Key, "orcon."):
			r.errorf(CodeExt, path, "extension key %q uses the reserved orcon. namespace", m.Key)
		case !extKeyPattern.MatchString(m.Key):
			r.errorf(CodeExt, path, "extension key %q must be namespaced as \"vendor.key\"", m.Key)
		}
	}
}

func checkObject(r *collector, path string, v jsonvalue.Value) {
	if !v.IsZero() && v.Kind() != jsonvalue.KindObject {
		r.errorf(CodeStructure, path, "must be an object, found %s", v.Kind())
	}
}

// structure runs the first validation level: the closed value sets, ranges
// and patterns that the JSON Schema also expresses.
func (v *validator) structure() {
	d, r := v.doc, v.r
	if d.Format != FormatTag {
		r.errorf(CodeFormat, "/format", "format %q is not %q", d.Format, FormatTag)
	}
	checkPattern(r, "/id", "flow id", string(d.ID), uuidPattern)
	if n := utf8.RuneCountInString(d.Name); n == 0 || n > maxFlowNameRunes {
		r.errorf(CodeValue, "/name", "name must have 1 to %d characters", maxFlowNameRunes)
	}
	checkObject(r, "/editor", d.Editor)
	checkExt(r, "/ext", d.Ext)
	v.settings()
	for i, vr := range d.Variables {
		v.variable(i, vr)
	}
	for i := range d.Nodes {
		n := &d.Nodes[i]
		v.node(i, n)
	}
	for i, e := range d.Edges {
		v.edge(i, e)
	}
}

func (v *validator) settings() {
	s, r := v.doc.Settings, v.r
	checkOptEnum(r, "/settings/branchMode", s.BranchMode, BranchModes())
	checkOptEnum(r, "/settings/priority", s.Priority, Priorities())
	checkOptEnum(r, "/settings/retention", s.Retention, Retentions())
	if s.Concurrency < 0 {
		r.errorf(CodeValue, "/settings/concurrency", "must be at least 1")
	}
	if s.MemoryLimit < 0 {
		r.errorf(CodeValue, "/settings/memoryLimit", "must be a positive number of bytes")
	}
	if s.CallDepthLimit < 0 {
		r.errorf(CodeValue, "/settings/callDepthLimit", "must be at least 1")
	}
	if s.ErrorFlow != "" {
		checkPattern(r, "/settings/errorFlow", "flow id", string(s.ErrorFlow), uuidPattern)
	}
	if s.Timeouts != nil {
		checkDuration(r, "/settings/timeouts/executionActive", s.Timeouts.ExecutionActive)
	}
}

func (v *validator) variable(i int, vr Variable) {
	r, base := v.r, ptr("variables", i)
	checkPattern(r, base+"/name", "variable name", vr.Name, variablePattern)
	checkEnum(r, base+"/scope", vr.Scope, VarScopes())
	checkOptEnum(r, base+"/accumulator", vr.Accumulator, Accumulators())
	if isTrue(vr.Carry) && vr.Scope != ScopeIteration {
		r.errorf(CodeValue, base+"/carry", "carry applies to iteration-scope variables only")
	}
}

func (v *validator) edge(i int, e Edge) {
	r, base := v.r, ptr("edges", i)
	checkPattern(r, base+"/id", "edge id", string(e.ID), idPattern)
	checkPattern(r, base+"/from/node", "node id", string(e.From.Node), idPattern)
	checkPattern(r, base+"/to/node", "node id", string(e.To), idPattern)
	if len(e.From.Port) > maxPortLength || !portPattern.MatchString(e.From.Port) {
		r.errorf(CodeValue, base+"/from/port", "port %q is not a valid port name", e.From.Port)
	}
	checkEnum(r, base+"/kind", e.Kind, EdgeKinds())
	if e.Order < 0 {
		r.errorf(CodeValue, base+"/order", "must be at least 1")
	}
}

func (v *validator) node(i int, n *Node) {
	r, base := v.r, ptr("nodes", i)
	checkPattern(r, base+"/id", "node id", string(n.ID), idPattern)
	if c := utf8.RuneCountInString(n.Name); c == 0 || c > maxNodeNameRunes {
		r.errorf(CodeValue, base+"/name", "name must have 1 to %d characters", maxNodeNameRunes)
	}
	checkEnum(r, base+"/element", n.Element, Elements())
	checkPattern(r, base+"/type", "node type", n.Type, typePattern)
	checkObject(r, base+"/parameters", n.Parameters)
	checkExt(r, base+"/ext", n.Ext)
	checkOptEnum(r, base+"/onError", n.OnError, OnErrors())
	v.routingAndLoop(base, n)
	v.variablesShape(i, n)
	v.nodeBlocks(base, n)
	v.waitAndBoundary(base, n)
	if n.Attention != nil {
		checkEnum(r, base+"/attention/notify", n.Attention.Notify, AttentionNotifies())
	}
}

// typeName returns the "<namespace>.<name>" part of a node type.
func typeName(t string) string {
	name, _, _ := strings.Cut(t, "@")
	return name
}

const (
	typeLoop   = "orcon.loop"
	typeIf     = "orcon.if"
	typeSwitch = "orcon.switch"
)

func (v *validator) routingAndLoop(base string, n *Node) {
	r := v.r
	if n.Routing != "" {
		checkEnum(r, base+"/routing", n.Routing, Routings())
		if t := typeName(n.Type); t != typeIf && t != typeSwitch {
			r.errorf(CodeBlock, base+"/routing", "routing applies to item-routing nodes (%s, %s), not to %q", typeIf, typeSwitch, n.Type)
		}
	}
	if n.Loop != nil && typeName(n.Type) != typeLoop {
		r.errorf(CodeBlock, base+"/loop", "the loop block applies to %s nodes only, not to %q", typeLoop, n.Type)
	}
}

func (v *validator) nodeBlocks(base string, n *Node) {
	r := v.r
	if j := n.Join; j != nil {
		checkEnum(r, base+"/join/mode", j.Mode, JoinModes())
		checkEnum(r, base+"/join/data", j.Data, JoinDataModes())
		checkOptEnum(r, base+"/join/remaining", j.Remaining, JoinRemainings())
		if j.Data == JoinDataMatchField && j.MatchField == "" {
			r.errorf(CodeValue, base+"/join/matchField", "matchField is required when data is %s", JoinDataMatchField)
		}
		if j.Data != JoinDataMatchField && j.MatchField != "" {
			r.errorf(CodeValue, base+"/join/matchField", "matchField applies only when data is %s", JoinDataMatchField)
		}
	}
	if m := n.MultiInstance; m != nil {
		if m.BatchSize < 1 {
			r.errorf(CodeValue, base+"/multiInstance/batchSize", "must be at least 1")
		}
		v.intOrKeyword(base+"/multiInstance/concurrency", m.Concurrency, KeywordAll)
	}
	if rt := n.Retry; rt != nil {
		if rt.MaxAttempts < 1 {
			r.errorf(CodeValue, base+"/retry/maxAttempts", "must be at least 1")
		}
		if rt.Factor != 0 && !(rt.Factor >= 1 && !math.IsInf(rt.Factor, 1)) {
			r.errorf(CodeValue, base+"/retry/factor", "must be a finite number of at least 1")
		}
		checkDuration(r, base+"/retry/initialInterval", rt.InitialInterval)
		checkDuration(r, base+"/retry/maxInterval", rt.MaxInterval)
	}
	if t := n.Timeouts; t != nil {
		checkDuration(r, base+"/timeouts/attempt", t.Attempt)
		checkDuration(r, base+"/timeouts/node", t.Node)
	}
	v.intOrKeyword(base+"/concurrency", n.Concurrency, KeywordAll)
	if c := n.Call; c != nil {
		v.call(base+"/call", n, c)
	}
}

// intOrKeyword checks a field that is a positive integer or one keyword.
func (v *validator) intOrKeyword(path string, k *IntOrKeyword, keyword string) {
	if k == nil {
		return
	}
	switch {
	case k.Keyword != "" && k.N != 0:
		v.r.errorf(CodeValue, path, "must be a number or %q, not both", keyword)
	case k.Keyword != "":
		if k.Keyword != keyword {
			checkEnum(v.r, path, k.Keyword, []string{keyword})
		}
	case k.N < 1:
		v.r.errorf(CodeValue, path, "must be at least 1 or %q", keyword)
	}
}

func (v *validator) call(path string, n *Node, c *Call) {
	r := v.r
	if n.Element != ElementCallActivity {
		r.errorf(CodeBlock, path, "the call block applies to %s nodes, not to %s", ElementCallActivity, n.Element)
	}
	if c.Target == "" {
		r.errorf(CodeValue, path+"/target", "target must name a flow or be an expression")
	} else if !strings.HasPrefix(c.Target, "=") {
		checkPattern(r, path+"/target", "flow id", c.Target, uuidPattern)
	}
	for i, t := range c.AllowedTargets {
		checkPattern(r, fmt.Sprintf("%s/allowedTargets/%d", path, i), "flow id", string(t), uuidPattern)
	}
	checkEnum(r, path+"/mode", c.Mode, CallModes())
	v.intOrKeyword(path+"/version", &c.Version, KeywordCurrent)
	if c.IntentionalRecursion != nil && c.IntentionalRecursion.MaxDepth < 1 {
		r.errorf(CodeValue, path+"/intentionalRecursion/maxDepth", "must be at least 1")
	}
}

func (v *validator) waitAndBoundary(base string, n *Node) {
	r := v.r
	if w := n.Wait; w != nil {
		path := base + "/wait"
		switch n.Element {
		case ElementReceiveTask, ElementUserTask, ElementIntermediateCatchEvent:
		case ElementStartEvent, ElementEndEvent, ElementTask, ElementServiceTask, ElementScriptTask,
			ElementSendTask, ElementCallActivity, ElementExclusiveGateway, ElementInclusiveGateway,
			ElementParallelGateway, ElementEventBasedGateway, ElementComplexGateway,
			ElementIntermediateThrowEvent, ElementBoundaryEvent:
			r.errorf(CodeBlock, path, "the wait block applies to receiveTask, userTask and intermediateCatchEvent nodes, not to %s", n.Element)
		default:
			// An element outside the closed set is reported by the enum check.
		}
		checkEnum(r, path+"/kind", w.Kind, WaitKinds())
		checkDuration(r, path+"/timeout", w.Timeout)
		checkObject(r, path+"/form", w.Form)
		v.waitTimer(path, w)
	}
	if n.Element == ElementBoundaryEvent {
		v.boundary(base, n)
		return
	}
	if n.AttachedTo != "" || n.CancelActivity != nil || n.EventDefinition != nil {
		r.errorf(CodeBlock, base, "attachedTo, cancelActivity and eventDefinition apply to boundaryEvent nodes only")
	}
}

func (v *validator) waitTimer(path string, w *Wait) {
	r := v.r
	switch {
	case w.Kind == WaitTimer && w.Timer == nil:
		r.errorf(CodeValue, path+"/timer", "a TIMER wait needs a timer with a duration or a date")
	case w.Kind != WaitTimer && w.Timer != nil:
		r.errorf(CodeBlock, path+"/timer", "timer applies to TIMER waits only")
	case w.Timer != nil:
		if w.Timer.Cycle != "" {
			r.errorf(CodeValue, path+"/timer/cycle", "a wait timer is a duration or a date; cycles belong to boundary events")
		}
		v.timer(path+"/timer", w.Timer, false)
	}
}

// timer checks that exactly one of duration, date and (when allowed) cycle is
// set and well formed.
func (v *validator) timer(path string, t *Timer, allowCycle bool) {
	r := v.r
	set := 0
	for _, s := range []string{t.Duration, t.Date, t.Cycle} {
		if s != "" {
			set++
		}
	}
	if set != 1 {
		choices := "duration or date"
		if allowCycle {
			choices = "duration, date or cycle"
		}
		r.errorf(CodeValue, path, "set exactly one of %s", choices)
	}
	checkDuration(r, path+"/duration", t.Duration)
	if t.Date != "" {
		if _, err := time.Parse(time.RFC3339, t.Date); err != nil || !instantPattern.MatchString(t.Date) {
			r.errorf(CodeValue, path+"/date", "%q is not an RFC 3339 date-time in UTC written with a Z, such as 2026-10-12T10:00:00Z", t.Date)
		}
	}
	if t.Cycle != "" {
		checkCycle(r, path+"/cycle", t.Cycle)
	}
}

func (v *validator) boundary(base string, n *Node) {
	r := v.r
	if n.AttachedTo == "" {
		r.errorf(CodeReference, base+"/attachedTo", "a boundaryEvent must name the activity it is attached to")
	} else {
		checkPattern(r, base+"/attachedTo", "node id", string(n.AttachedTo), idPattern)
	}
	if n.CancelActivity == nil {
		r.errorf(CodeValue, base+"/cancelActivity", "a boundaryEvent must say whether it cancels the activity")
	}
	ed := n.EventDefinition
	if ed == nil {
		r.errorf(CodeValue, base+"/eventDefinition", "a boundaryEvent needs an eventDefinition")
		return
	}
	set := 0
	if ed.Timer != nil {
		set++
		v.timer(base+"/eventDefinition/timer", ed.Timer, true)
	}
	if ed.Error != nil {
		set++
	}
	if ed.Compensation {
		set++
	}
	if set != 1 {
		r.errorf(CodeValue, base+"/eventDefinition", "set exactly one of timer, error and compensation")
	}
}

// references runs the second level: ids, names, ports and variable names
// must point at something that exists, and must be unique where they name.
func (v *validator) references() {
	d, r := v.doc, v.r
	nodes := make(map[NodeID]*Node, len(d.Nodes))
	names := make(map[string]NodeID, len(d.Nodes))
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if _, dup := nodes[n.ID]; dup {
			r.errorf(CodeReference, ptr("nodes", i, "id"), "node id %q is used more than once", n.ID)
		}
		nodes[n.ID] = n
		if first, dup := names[n.Name]; dup {
			r.errorf(CodeV12, ptr("nodes", i, "name"), "node name %q is already used by node %q; names are used by $('Name') and must be unique", n.Name, first)
		}
		names[n.Name] = n.ID
	}
	declared := make(map[string]Variable, len(d.Variables))
	for i, vr := range d.Variables {
		if _, dup := declared[vr.Name]; dup {
			r.errorf(CodeReference, ptr("variables", i, "name"), "variable %q is declared more than once", vr.Name)
		}
		declared[vr.Name] = vr
	}
	edges := make(map[EdgeID]struct{}, len(d.Edges))
	for i, e := range d.Edges {
		if _, dup := edges[e.ID]; dup {
			r.errorf(CodeReference, ptr("edges", i, "id"), "edge id %q is used more than once", e.ID)
		}
		edges[e.ID] = struct{}{}
		v.edgeRefs(i, e, nodes)
	}
	for i := range d.Nodes {
		n := &d.Nodes[i]
		v.nodeRefs(i, n, nodes, declared)
	}
}

func (v *validator) edgeRefs(i int, e Edge, nodes map[NodeID]*Node) {
	if _, ok := nodes[e.From.Node]; !ok {
		v.r.errorf(CodeReference, ptr("edges", i, "from", "node"), "edge %q starts at unknown node %q", e.ID, e.From.Node)
	}
	if _, ok := nodes[e.To]; !ok {
		v.r.errorf(CodeReference, ptr("edges", i, "to", "node"), "edge %q ends at unknown node %q", e.ID, e.To)
	}
}

func (v *validator) nodeRefs(i int, n *Node, nodes map[NodeID]*Node, declared map[string]Variable) {
	r, base := v.r, ptr("nodes", i)
	if n.Element == ElementBoundaryEvent && n.AttachedTo != "" {
		if _, ok := nodes[n.AttachedTo]; !ok {
			r.errorf(CodeReference, base+"/attachedTo", "boundary event %q is attached to unknown node %q", n.ID, n.AttachedTo)
		}
	}
	if c := n.Compensation; c != nil {
		h, ok := nodes[c.Handler]
		switch {
		case !ok:
			r.errorf(CodeReference, base+"/compensation/handler", "compensation handler %q is not a node of this flow", c.Handler)
		case h.Compensation != nil:
			r.errorf(CodeReference, base+"/compensation/handler", "compensation handler %q has a compensation of its own; compensations are not themselves compensated", c.Handler)
		}
	}
	for _, w := range v.writers(n) {
		if _, ok := declared[w]; !ok {
			r.errorf(CodeReference, ptr("nodes", i), "node %q writes undeclared variable %q", n.ID, w)
		}
	}
	if n.Loop != nil {
		for j, c := range n.Loop.Carry {
			vr, ok := declared[c]
			switch {
			case !ok:
				r.errorf(CodeReference, ptr("nodes", i, "loop", "carry", j), "loop carries undeclared variable %q", c)
			case vr.Scope != ScopeIteration || !isTrue(vr.Carry):
				r.errorf(CodeReference, ptr("nodes", i, "loop", "carry", j), "variable %q is carried but is not an iteration variable with carry: true", c)
			}
		}
	}
}
