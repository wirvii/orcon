// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package conformance

import (
	"flag"
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

// storeFlag returns the engine under test, like the -store of the other conformance
// criteria: memory is the in-memory interpreter, session the session driver over a
// fake database (a development check of the snapshot and the delta). The durable
// stores are not reachable from here: an area test does not import an adapter
// (AL-2.1), so the suite runs on them from test/engine when the cut that repoints
// the conformance criteria wires it.
func storeFlag() string { return executiontest.StoreFlag("memory") }

// requireStore fails the calling test when the selected store is not wired.
// It never skips: a skipped suite exits 0 without running anything, and the
// command of criterion E-CF-1 would then look satisfied.
func requireStore(t *testing.T) {
	t.Helper()
	switch s := storeFlag(); s {
	case "memory", "session":
	case "sqlite", "postgres", "both", "auto":
		t.Fatalf("store %s: the conformance suite does not run on the durable stores from here yet", s)
	default:
		t.Fatalf("store %q: want memory, session, sqlite, postgres or both", s)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	if storeFlag() == "session" {
		executiontest.UseEngine(executiontest.SessionEngine)
	}
	m.Run()
}

// Status of a pattern in the matrix.
const (
	supported   = "supported"
	unsupported = "unsupported"
	// The later status is an internal development status for a pattern a later cut or an
	// owner decision completes. ADR-0019 publishes only supported and
	// unsupported(reason), so the generation of the published matrix (cut C16)
	// must fail while any row is still later.
	later = "later"
)

// pattern is one row of the Workflow Control-Flow Patterns matrix.
type pattern struct {
	id     int
	name   string
	status string
	// note explains a supported pattern's mapping, names the cut that
	// provides a later one, or gives the reason of an unsupported one.
	note string
	run  func(t *testing.T)
}

// runFlow runs a flow on the engine under test.
func runFlow(t *testing.T, b *executiontest.Builder, deps execution.Deps, input execution.Batch) execution.Result {
	t.Helper()
	requireStore(t)
	res := executiontest.Run(t, b, deps, input)
	if res.Status != execution.ExecSucceeded {
		t.Fatalf("status = %s, want SUCCEEDED (failure %v)", res.Status, res.Failure)
	}
	return res
}

func tasks(runner *executiontest.TraceRunner) []string {
	var out []string
	for _, id := range runner.Calls() {
		out = append(out, string(id))
	}
	return out
}

func sorted(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}

func one() execution.Batch { return executiontest.Batch(`{"id":1}`) }

// minimumSet is the set of patterns ADR-0019 section 1 requires for release.
var minimumSet = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 16, 19, 20, 21, 22, 24, 25, 26, 28, 29, 33, 34, 35, 37, 41, 42, 43}

// matrix lists WCP 1-43. The rows of the minimum set that are not supported
// yet name the cut or decision that completes them.
func matrix() []pattern {
	return []pattern{
		{1, "Sequence", supported, "task chain", wcp01Sequence},
		{2, "Parallel Split", supported, "parallelGateway with several outgoing edges", wcp02ParallelSplit},
		{3, "Synchronization", supported, "join ALL", wcp03Synchronization},
		{4, "Exclusive Choice", supported, "exclusiveGateway", wcp04ExclusiveChoice},
		{5, "Simple Merge", supported, "uncontrolled merge after an exclusive choice", wcp05SimpleMerge},
		{6, "Multi-Choice", supported, "inclusiveGateway", wcp06MultiChoice},
		{7, "Structured Synchronizing Merge", supported, "join ACTIVATED after an inclusive gateway", wcp07StructuredSynchronizingMerge},
		{8, "Multi-Merge", supported, "uncontrolled merge after a parallel split", wcp08MultiMerge},
		{9, "Structured Discriminator", supported, "join FIRST with remaining LET_FINISH", wcp09StructuredDiscriminator},
		{10, "Arbitrary Cycles", supported, "cycles through an orcon.loop control, which may be left from several places of the body", wcp10ArbitraryCycles},
		{11, "Implicit Termination", supported, "the execution ends when no live token is left", wcp11ImplicitTermination},
		{12, "Multiple Instances without Synchronization", later, "cut C9", nil},
		{13, "Multiple Instances with a Priori Design-Time Knowledge", later, "cut C9", nil},
		{14, "Multiple Instances with a Priori Run-Time Knowledge", later, "cut C9", nil},
		{15, "Multiple Instances without a Priori Run-Time Knowledge", later, "cut C9", nil},
		{16, "Deferred Choice", later, "cut C8 (event-based gateway)", nil},
		{17, "Interleaved Parallel Routing", unsupported, "SEQUENTIAL branch mode fixes the order in the edges; a runtime choice among orders is not defined", nil},
		{18, "Milestone", unsupported, "no construct enables an activity from the state of another branch", nil},
		{19, "Cancel Task", later, "cut C8 (interrupting boundary event)", nil},
		{20, "Cancel Case", later, "cut C7 (cancellation)", nil},
		{21, "Structured Loop", supported, "orcon.loop with the exit test before or after the body", wcp21StructuredLoop},
		{22, "Recursion", later, "cut C9 (sub-flow call activity)", nil},
		{23, "Transient Trigger", unsupported, "signals are durable (ADR-0010 section 8), so a trigger is never lost", nil},
		{24, "Persistent Trigger", later, "cut C8 (durable signal wait)", nil},
		{25, "Cancel Region", later, "cut C7 (cancellation)", nil},
		{26, "Cancel Multiple Instance Activity", later, "cut C9", nil},
		{27, "Complete Multiple Instance Activity", later, "cut C9 (completionCondition)", nil},
		{28, "Blocking Discriminator", supported, "join FIRST with remaining LET_FINISH: a second round waits for the first to reset", wcp28BlockingDiscriminator},
		{29, "Canceling Discriminator", supported, "join FIRST with remaining CANCEL", wcp29CancelingDiscriminator},
		{30, "Structured Partial Join", unsupported, "joins fire on all, any live or the first; N-of-M joins are not defined", nil},
		{31, "Blocking Partial Join", unsupported, "N-of-M joins are not defined", nil},
		{32, "Canceling Partial Join", unsupported, "N-of-M joins are not defined", nil},
		{33, "Generalised AND-Join", supported, "join ALL with several live tokens on one edge", wcp33GeneralisedAndJoin},
		{34, "Static Partial Join for Multiple Instances", later, "cut C9", nil},
		{35, "Canceling Partial Join for Multiple Instances", later, "cut C9", nil},
		{36, "Dynamic Partial Join for Multiple Instances", unsupported, "partial joins are not defined", nil},
		{37, "Local Synchronizing Merge", supported, "join ACTIVATED decided by local completion signals", wcp37LocalSynchronizingMerge},
		{38, "General Synchronizing Merge", unsupported, "ADR-0010 rejects the global reachability analysis it needs; OR-joins are local", nil},
		{39, "Critical Section", unsupported, "no mutual exclusion construct", nil},
		{40, "Interleaved Routing", unsupported, "no mutual exclusion construct", nil},
		{41, "Thread Merge", supported, "join ALL merging the threads of one branch into one", wcp41ThreadMerge},
		{42, "Thread Split", supported, "parallel split inside a branch", wcp42ThreadSplit},
		{43, "Explicit Termination", later, "cut C7 (terminate end event)", nil},
	}
}

// TestMinimumSetNeverUnsupported keeps the release set honest: a row of the
// ADR-0019 minimum set may be supported or later, never unsupported.
func TestMinimumSetNeverUnsupported(t *testing.T) {
	t.Parallel()
	byID := make(map[int]pattern)
	for _, p := range matrix() {
		byID[p.id] = p
	}
	for _, id := range minimumSet {
		if p := byID[id]; p.status == unsupported {
			t.Errorf("WCP %d (%s) is in the release minimum set and is marked unsupported: %s", id, p.name, p.note)
		}
	}
}

func TestPatternMatrixIsComplete(t *testing.T) {
	t.Parallel()
	m := matrix()
	if len(m) != 43 {
		t.Fatalf("matrix has %d rows, want 43", len(m))
	}
	for i, p := range m {
		if p.id != i+1 {
			t.Errorf("row %d is WCP %d, want WCP %d", i, p.id, i+1)
		}
		if p.name == "" || p.note == "" {
			t.Errorf("WCP %d has no name or note", p.id)
		}
		switch p.status {
		case supported:
			if p.run == nil {
				t.Errorf("WCP %d is supported and has no test", p.id)
			}
		case later, unsupported:
			if p.run != nil {
				t.Errorf("WCP %d is %s and has a test", p.id, p.status)
			}
		default:
			t.Errorf("WCP %d has status %q", p.id, p.status)
		}
	}
}

// TestWorkflowPatterns runs one test per pattern. A pattern that is not
// supported yet is reported as skipped with its status and reason.
func TestWorkflowPatterns(t *testing.T) {
	t.Parallel()
	requireStore(t)
	for _, p := range matrix() {
		t.Run(fmt.Sprintf("WCP%02d_%s", p.id, p.name), func(t *testing.T) {
			t.Parallel()
			if p.status != supported {
				t.Skipf("%s: %s", p.status, p.note)
			}
			requireStore(t)
			p.run(t)
		})
	}
}

func wcp01Sequence(t *testing.T) {
	t.Helper()
	runner := &executiontest.TraceRunner{}
	b := executiontest.NewFlow("wcp1").Start("s").Task("a").Task("b").Task("c").End("e").Chain("s", "a", "b", "c", "e")
	res := runFlow(t, b, execution.Deps{Runner: runner}, one())
	if diff := cmp.Diff([]string{"a", "b", "c"}, tasks(runner)); diff != "" {
		t.Errorf("order mismatch (-want +got):\n%s", diff)
	}
	if got := executiontest.Paths(res); !slices.Equal(got, []string{"end:a>b>c"}) {
		t.Errorf("paths = %v", got)
	}
}

func wcp02ParallelSplit(t *testing.T) {
	t.Helper()
	runner := &executiontest.TraceRunner{}
	b := executiontest.NewFlow("wcp2").Start("s").Parallel("p").Task("a").Task("b").End("ea").End("eb").
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "ea").Edge("b", "eb")
	runFlow(t, b, execution.Deps{Runner: runner}, one())
	if diff := cmp.Diff([]string{"a", "b"}, sorted(tasks(runner))); diff != "" {
		t.Errorf("both branches must run once (-want +got):\n%s", diff)
	}
}

func syncFlow(mode flow.JoinMode) *executiontest.Builder {
	return executiontest.NewFlow("sync").Start("s").Parallel("p").Task("a").Task("b").Parallel("j").Task("after").End("e").
		Join("j", mode).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Chain("j", "after", "e")
}

func wcp03Synchronization(t *testing.T) {
	t.Helper()
	runner := &executiontest.TraceRunner{}
	res := runFlow(t, syncFlow(flow.JoinAll), execution.Deps{Runner: runner}, one())
	if diff := cmp.Diff([]string{"a", "b", "after"}, tasks(runner)); diff != "" {
		t.Errorf("the node after the join runs once, after both branches (-want +got):\n%s", diff)
	}
	if executiontest.Executed(res, "after") != 1 {
		t.Errorf("after ran %d times, want 1", executiontest.Executed(res, "after"))
	}
}

func choiceFlow() *executiontest.Builder {
	return executiontest.NewFlow("choice").Start("s").Exclusive("x").Task("a").Task("b").Task("m").End("e").
		Edge("s", "x").Edge("x", "a", executiontest.Cond("a")).Edge("x", "b", executiontest.Default()).
		Edge("a", "m").Edge("b", "m").Chain("m", "e")
}

func wcp04ExclusiveChoice(t *testing.T) {
	t.Helper()
	for _, pick := range []bool{true, false} {
		runner := &executiontest.TraceRunner{}
		runFlow(t, choiceFlow(), execution.Deps{Runner: runner, Conditions: executiontest.Conditions{"a": pick}}, one())
		want := []string{"b", "m"}
		if pick {
			want = []string{"a", "m"}
		}
		if diff := cmp.Diff(want, tasks(runner)); diff != "" {
			t.Errorf("condition a=%v: exactly one branch runs (-want +got):\n%s", pick, diff)
		}
	}
}

func wcp05SimpleMerge(t *testing.T) {
	t.Helper()
	for _, pick := range []bool{true, false} {
		res := runFlow(t, choiceFlow(), execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: executiontest.Conditions{"a": pick}}, one())
		if got := executiontest.Executed(res, "m"); got != 1 {
			t.Errorf("condition a=%v: the merge ran %d times, want 1", pick, got)
		}
	}
}

func orFlow(mode flow.JoinMode) *executiontest.Builder {
	return executiontest.NewFlow("or").Start("s").Inclusive("o").Task("a").Task("b").Task("c").Parallel("j").Task("after").End("e").
		Join("j", mode).
		Edge("s", "o").
		Edge("o", "a", executiontest.Cond("a")).Edge("o", "b", executiontest.Cond("b")).Edge("o", "c", executiontest.Default()).
		Edge("a", "j").Edge("b", "j").Edge("c", "j").Chain("j", "after", "e")
}

func wcp06MultiChoice(t *testing.T) {
	t.Helper()
	tests := []struct {
		conds executiontest.Conditions
		want  []string
	}{
		{executiontest.Conditions{"a": true, "b": true}, []string{"a", "b"}},
		{executiontest.Conditions{"a": true, "b": false}, []string{"a"}},
		{executiontest.Conditions{"a": false, "b": true}, []string{"b"}},
		{executiontest.Conditions{"a": false, "b": false}, []string{"c"}},
	}
	for _, tt := range tests {
		runner := &executiontest.TraceRunner{}
		runFlow(t, orFlow(flow.JoinActivated), execution.Deps{Runner: runner, Conditions: tt.conds}, one())
		got := slices.DeleteFunc(sorted(tasks(runner)), func(s string) bool { return s == "after" })
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("conditions %v: the taken subset runs (-want +got):\n%s", tt.conds, diff)
		}
	}
}

func wcp07StructuredSynchronizingMerge(t *testing.T) {
	t.Helper()
	for _, conds := range []executiontest.Conditions{
		{"a": true, "b": true}, {"a": true, "b": false}, {"a": false, "b": false},
	} {
		res := runFlow(t, orFlow(flow.JoinActivated), execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: conds}, one())
		if got := executiontest.Executed(res, "after"); got != 1 {
			t.Errorf("conditions %v: the node after the merge ran %d times, want 1 (the merge waits for the taken branches only)", conds, got)
		}
	}
}

func wcp08MultiMerge(t *testing.T) {
	t.Helper()
	b := executiontest.NewFlow("wcp8").Start("s").Parallel("p").Task("a").Task("b").Task("m").End("e").
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "m").Edge("b", "m").Chain("m", "e")
	res := runFlow(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, one())
	if got := executiontest.Executed(res, "m"); got != 2 {
		t.Errorf("the merge ran %d times, want once per incoming token (2)", got)
	}
}

func wcp09StructuredDiscriminator(t *testing.T) {
	t.Helper()
	runner := &executiontest.TraceRunner{}
	res := runFlow(t, syncFlow(flow.JoinFirst).Remaining("j", flow.RemainingLetFinish), execution.Deps{Runner: runner}, one())
	if diff := cmp.Diff([]string{"a", "after", "b"}, sorted(tasks(runner))); diff != "" {
		t.Errorf("every branch runs and the continuation runs once (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"end:a>after"}, executiontest.Paths(res)); diff != "" {
		t.Errorf("the continuation carries the first arrival only (-want +got):\n%s", diff)
	}
	if got := executiontest.TokenStates(res)[execution.TokenCanceled]; got != 0 {
		t.Errorf("canceled tokens = %d, want none: LET_FINISH cancels nothing", got)
	}
	var reset *execution.Event
	for i := range res.Events {
		if res.Events[i].Type == execution.EventJoinReset {
			reset = &res.Events[i]
		}
	}
	if reset == nil || reset.Round != 1 || reset.Arrivals != 2 {
		t.Errorf("join.reset = %+v, want round 1 after 2 arrivals", reset)
	}
}

// wcp28Flow has an uncontrolled merge in front of a split, so the split runs
// twice in the same scope and the discriminator sees two rounds.
func wcp28Flow() *executiontest.Builder {
	return executiontest.NewFlow("wcp28").
		Start("s").Parallel("p0").Task("a").Task("b").Task("m").Parallel("sp").Task("x").Task("y").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinFirst).Remaining("j", flow.RemainingLetFinish).
		Chain("s", "p0").Edge("p0", "a").Edge("p0", "b").Edge("a", "m").Edge("b", "m").Edge("m", "sp").
		Edge("sp", "x").Edge("sp", "y").Edge("x", "j").Edge("y", "j").Chain("j", "after", "e")
}

func wcp28BlockingDiscriminator(t *testing.T) {
	t.Helper()
	runner := &executiontest.TraceRunner{}
	res := runFlow(t, wcp28Flow(), execution.Deps{Runner: runner}, one())
	if got := executiontest.Executed(res, "after"); got != 2 {
		t.Fatalf("the node after the join ran %d times, want 2: one per round", got)
	}
	seq := func(typ execution.EventType, round int) int {
		for i := range res.Events {
			if e := &res.Events[i]; e.Type == typ && e.Round == round {
				return e.Sequence
			}
		}
		return 0
	}
	fired1, reset1 := seq(execution.EventJoinFired, 1), seq(execution.EventJoinReset, 1)
	fired2, reset2 := seq(execution.EventJoinFired, 2), seq(execution.EventJoinReset, 2)
	if fired1 == 0 || reset1 == 0 || fired2 == 0 || reset2 == 0 {
		t.Fatalf("join events missing: fired %d %d, reset %d %d", fired1, fired2, reset1, reset2)
	}
	if fired1 >= reset1 || reset1 >= fired2 || fired2 >= reset2 {
		t.Errorf("round order wrong: fired1=%d reset1=%d fired2=%d reset2=%d; round 2 must wait until round 1 reset", fired1, reset1, fired2, reset2)
	}
	if executiontest.Count(res, execution.EventJoinFired) != 2 || executiontest.Count(res, execution.EventJoinReset) != 2 {
		t.Errorf("join events: %d fired, %d reset, want 2 each", executiontest.Count(res, execution.EventJoinFired), executiontest.Count(res, execution.EventJoinReset))
	}
}

func wcp29CancelingDiscriminator(t *testing.T) {
	t.Helper()
	runner := &executiontest.TraceRunner{}
	res := runFlow(t, syncFlow(flow.JoinFirst).Remaining("j", flow.RemainingCancel), execution.Deps{Runner: runner}, one())
	if diff := cmp.Diff([]string{"a", "after"}, tasks(runner)); diff != "" {
		t.Errorf("the losing branch is canceled before it runs (-want +got):\n%s", diff)
	}
	if got := executiontest.TokenStates(res)[execution.TokenCanceled]; got != 1 {
		t.Errorf("canceled tokens = %d, want 1", got)
	}
}

// cyclesFlow has two exits from the body of one loop:
// start -> L -> w1 -> c1 -{stop}-> eEarly, c1 -{go}-> w2 -> c2 -{again}-> L (back), c2 -{done}-> eDone.
func cyclesFlow() *executiontest.Builder {
	return executiontest.NewFlow("wcp10").
		Start("s").Loop("L", 10).Task("w1").If("c1", flow.RoutingExclusive).Task("w2").If("c2", flow.RoutingExclusive).End("eEarly").End("eDone").
		Edge("s", "L").Edge("L", "w1", executiontest.Port("loop")).Edge("w1", "c1").
		Edge("c1", "eEarly", executiontest.Port("stop")).Edge("c1", "w2", executiontest.Port("go")).Edge("w2", "c2").
		Edge("c2", "L", executiontest.Port("again"), executiontest.Back()).Edge("c2", "eDone", executiontest.Port("done"))
}

func wcp10ArbitraryCycles(t *testing.T) {
	t.Helper()
	router := executiontest.RouterFunc(func(n *flow.Node, it execution.Item) []string {
		if n.ID == "c1" {
			if v, ok := it.JSON.Get("stopEarly"); ok {
				if stop, _ := v.Bool(); stop {
					return []string{"stop"}
				}
			}
			return []string{"go"}
		}
		if p := executiontest.WhileLess("n", 3)(n, it); p[0] == "true" {
			return []string{"again"}
		}
		return []string{"done"}
	})
	deps := func() execution.Deps {
		return execution.Deps{
			Router: router,
			Runner: &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"w2": executiontest.Increment("n")}},
		}
	}
	early := runFlow(t, cyclesFlow(), deps(), executiontest.Batch(`{"n":0,"stopEarly":true}`))
	if len(early.Ends) != 1 || early.Ends[0].Node != "eEarly" {
		t.Errorf("stopEarly: ends = %+v, want one arrival at eEarly", early.Ends)
	}
	full := runFlow(t, cyclesFlow(), deps(), executiontest.Batch(`{"n":0}`))
	if len(full.Ends) != 1 || full.Ends[0].Node != "eDone" || executiontest.Executed(full, "w2") != 3 {
		t.Errorf("full run: ends = %+v with %d w2 runs, want one arrival at eDone after 3 iterations", full.Ends, executiontest.Executed(full, "w2"))
	}
}

func wcp11ImplicitTermination(t *testing.T) {
	t.Helper()
	// Branch a ends in a node with no outgoing edge and there is no end event
	// on it: the execution still ends, successfully, when nothing is live.
	b := executiontest.NewFlow("wcp11").Start("s").Parallel("p").Task("a").Task("b").End("e").
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("b", "e")
	runner := &executiontest.TraceRunner{}
	res := runFlow(t, b, execution.Deps{Runner: runner}, one())
	if diff := cmp.Diff([]string{"a", "b"}, sorted(tasks(runner))); diff != "" {
		t.Errorf("both branches must run (-want +got):\n%s", diff)
	}
	if len(res.Ends) != 1 {
		t.Errorf("end arrivals = %d, want 1 (only branch b reaches an end event)", len(res.Ends))
	}
}

// whileFlow tests the loop condition before the body: a pre-test loop.
func whileFlow(post bool) *executiontest.Builder {
	b := executiontest.NewFlow("wcp21").Start("s").Loop("L", 10).Task("work").If("test", flow.RoutingExclusive).End("e")
	b.Edge("s", "L")
	if !post {
		b.Edge("L", "test", executiontest.Port("loop")).Edge("test", "work", executiontest.Port("true")).Edge("work", "L", executiontest.Back())
		return b.Edge("test", "e", executiontest.Port("false"))
	}
	b.Edge("L", "work", executiontest.Port("loop")).Edge("work", "test")
	return b.Edge("test", "L", executiontest.Port("true"), executiontest.Back()).Edge("test", "e", executiontest.Port("false"))
}

func wcp21StructuredLoop(t *testing.T) {
	t.Helper()
	deps := func() execution.Deps {
		return execution.Deps{
			Router: executiontest.WhileLess("n", 3),
			Runner: &executiontest.TraceRunner{Behaviors: map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error){"work": executiontest.Increment("n")}},
		}
	}
	post := runFlow(t, whileFlow(true), deps(), executiontest.Batch(`{"n":0}`))
	if got := executiontest.Executed(post, "work"); got != 3 {
		t.Errorf("post-test loop ran the body %d times, want 3", got)
	}
	postAlready := runFlow(t, whileFlow(true), deps(), executiontest.Batch(`{"n":7}`))
	if got := executiontest.Executed(postAlready, "work"); got != 1 {
		t.Errorf("post-test loop ran the body %d times with the condition already false, want 1 (do-while)", got)
	}
	pre := runFlow(t, whileFlow(false), deps(), executiontest.Batch(`{"n":0}`))
	if got := executiontest.Executed(pre, "work"); got != 3 {
		t.Errorf("pre-test loop ran the body %d times, want 3", got)
	}
	preAlready := runFlow(t, whileFlow(false), deps(), executiontest.Batch(`{"n":7}`))
	if got := executiontest.Executed(preAlready, "work"); got != 0 {
		t.Errorf("pre-test loop ran the body %d times with the condition already false, want 0 (while)", got)
	}
}

func wcp33GeneralisedAndJoin(t *testing.T) {
	t.Helper()
	// The edge m -> j carries two live tokens (m is an uncontrolled merge that
	// activates twice). The AND-join waits for the whole edge, fires once, and
	// takes both tokens.
	b := executiontest.NewFlow("wcp33").Start("s").Parallel("p").Task("x").Task("y").Task("z").Task("m").Parallel("j").End("e").
		Join("j", flow.JoinAll).
		Chain("s", "p").Edge("p", "x").Edge("p", "y").Edge("p", "z").
		Edge("x", "m").Edge("y", "m").Edge("m", "j").Edge("z", "j").Chain("j", "e")
	res := runFlow(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, one())
	if got := executiontest.Executed(res, "j"); got != 1 {
		t.Errorf("the AND-join ran %d times, want 1", got)
	}
	if len(res.Ends) != 1 || res.Ends[0].Batch.Len() != 3 {
		t.Errorf("ends = %+v, want one arrival with the 3 items of both tokens and the other edge", res.Ends)
	}
}

func wcp37LocalSynchronizingMerge(t *testing.T) {
	t.Helper()
	// The merge decides from local completion signals only. Branch a contains
	// an exclusive choice whose untaken path also leads to the merge; the merge
	// must wait for the choice to resolve, and not for the b branch when the
	// inclusive split did not take it.
	b := executiontest.NewFlow("wcp37").
		Start("s").Inclusive("o").Task("a").Exclusive("x").Task("a1").Task("a2").Task("b").Parallel("j").Task("after").End("e").
		Join("j", flow.JoinActivated).
		Edge("s", "o").Edge("o", "a", executiontest.Cond("a")).Edge("o", "b", executiontest.Cond("b")).
		Edge("a", "x").Edge("x", "a1", executiontest.Cond("left")).Edge("x", "a2", executiontest.Default()).
		Edge("a1", "j").Edge("a2", "j").Edge("b", "j").Chain("j", "after", "e")
	for _, conds := range []executiontest.Conditions{
		{"a": true, "b": false, "left": true},
		{"a": true, "b": true, "left": false},
		{"a": false, "b": true, "left": false},
	} {
		res := runFlow(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}, Conditions: conds}, one())
		if got := executiontest.Executed(res, "after"); got != 1 {
			t.Errorf("conditions %v: the node after the merge ran %d times, want 1", conds, got)
		}
	}
}

func wcp41ThreadMerge(t *testing.T) {
	t.Helper()
	// A branch splits into two threads (y1, y2) and merges them back into one
	// before continuing; the node after the merge runs once, not per thread.
	b := executiontest.NewFlow("wcp41").
		Start("s").Parallel("p1").Task("x").Parallel("p2").Task("y1").Task("y2").Parallel("j2").Task("x2").Task("z").Parallel("j1").End("e").
		Join("j2", flow.JoinAll).Join("j1", flow.JoinAll).
		Chain("s", "p1").Edge("p1", "x").Edge("p1", "z").Edge("x", "p2").Edge("p2", "y1").Edge("p2", "y2").
		Edge("y1", "j2").Edge("y2", "j2").Edge("j2", "x2").Edge("x2", "j1").Edge("z", "j1").Edge("j1", "e")
	res := runFlow(t, b, execution.Deps{Runner: &executiontest.TraceRunner{}}, one())
	if got := executiontest.Executed(res, "x2"); got != 1 {
		t.Errorf("the node after the thread merge ran %d times, want 1", got)
	}
}

func wcp42ThreadSplit(t *testing.T) {
	t.Helper()
	// The split happens inside a branch of an outer split; no synchronization
	// follows, so the threads end separately.
	b := executiontest.NewFlow("wcp42").
		Start("s").Parallel("p1").Task("x").Parallel("p2").Task("t1").Task("t2").Task("z").End("e1").End("e2").End("e3").
		Chain("s", "p1").Edge("p1", "x").Edge("p1", "z").Edge("x", "p2").Edge("p2", "t1").Edge("p2", "t2").
		Edge("t1", "e1").Edge("t2", "e2").Edge("z", "e3")
	runner := &executiontest.TraceRunner{}
	res := runFlow(t, b, execution.Deps{Runner: runner}, one())
	if diff := cmp.Diff([]string{"t1", "t2", "x", "z"}, sorted(tasks(runner))); diff != "" {
		t.Errorf("every thread must run once (-want +got):\n%s", diff)
	}
	if len(res.Ends) != 3 {
		t.Errorf("end arrivals = %d, want 3 (one per thread)", len(res.Ends))
	}
}
