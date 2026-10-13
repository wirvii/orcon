// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// TestValidateAcceptsConformanceFlows covers the positive side of E-FMT-1:
// every conformance flow reads, validates without errors and warnings that
// are not intended, and reads back to the same JSON data.
func TestValidateAcceptsConformanceFlows(t *testing.T) {
	t.Parallel()
	for _, name := range conformanceNames(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw := conformanceValue(t, name)
			doc, err := flow.Read(raw)
			if err != nil {
				t.Fatalf("Read(%s) error: %v", name, err)
			}
			report := flow.Validate(&doc)
			if report.HasErrors() {
				t.Errorf("Validate(%s) has errors: %v", name, report.Errors())
			}
			if len(report.Warnings()) > 0 {
				t.Errorf("Validate(%s) has unexpected warnings: %v", name, report.Warnings())
			}
			a, errA := jsonvalue.Canonical(raw)
			b, errB := jsonvalue.Canonical(doc.Value())
			if errA != nil || errB != nil {
				t.Fatalf("Canonical errors: %v, %v", errA, errB)
			}
			if diff := cmp.Diff(string(a), string(b)); diff != "" {
				t.Errorf("document does not read back to the same JSON data (-file +model):\n%s", diff)
			}
		})
	}
}

// ruleCase is one negative case: starting from a valid conformance flow, a
// mutation breaks exactly one rule and the report must carry the code.
type ruleCase struct {
	name   string
	base   string
	mutate func(d *flow.Document)
	code   string
	sev    flow.Severity
}

func runRuleCases(t *testing.T, cases []ruleCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := load(t, tt.base)
			if r := flow.Validate(&doc); r.HasErrors() {
				t.Fatalf("base flow %s is not valid before the mutation: %v", tt.base, r.Errors())
			}
			tt.mutate(&doc)
			report := flow.Validate(&doc)
			if !has(report, tt.code, tt.sev) {
				t.Errorf("Validate after %q: want %s %s, got issues %v", tt.name, tt.sev, tt.code, report.Issues)
			}
		})
	}
}

func TestValidateControlFlowRules(t *testing.T) {
	t.Parallel()
	runRuleCases(t, []ruleCase{
		{"V1 orphan node", "linear", func(d *flow.Document) {
			d.Nodes = append(d.Nodes, flow.Node{ID: "orphan", Name: "Orphan", Element: flow.ElementTask, Type: "orcon.set@1", Parameters: obj()})
		}, flow.CodeV1, flow.SeverityError},
		{"V1 no start event", "linear", func(d *flow.Document) {
			node(d, "start").Element = flow.ElementTask
		}, flow.CodeV1, flow.SeverityError},
		{"V2 forward cycle", "loop", func(d *flow.Document) {
			edge(d, "e4").Kind = flow.EdgeSequence
		}, flow.CodeV2, flow.SeverityError},
		{"V2 back edge to a non-loop node", "loop", func(d *flow.Document) {
			edge(d, "e4").To = "page"
		}, flow.CodeV2, flow.SeverityError},
		{"V3 loop without block", "loop", func(d *flow.Document) {
			node(d, "loop").Loop = nil
		}, flow.CodeV3, flow.SeverityError},
		{"V3 loop without maxIterations", "loop", func(d *flow.Document) {
			node(d, "loop").Loop.MaxIterations = 0
		}, flow.CodeV3, flow.SeverityError},
		{"V4 signal wait without timeout duration", "waits-and-boundary", func(d *flow.Document) {
			node(d, "callback").Wait.Timeout = ""
		}, flow.CodeV4, flow.SeverityError},
		{"V4 human wait without timeout path", "waits-and-boundary", func(d *flow.Document) {
			dropEdge(d, "e6")
		}, flow.CodeV4, flow.SeverityError},
		{"V5 split without default is a warning", "exclusive-and-inclusive", func(d *flow.Document) {
			edge(d, "e3").Default = nil
		}, flow.CodeV5, flow.SeverityWarning},
		{"V5 two defaults", "exclusive-and-inclusive", func(d *flow.Document) {
			edge(d, "e2").Default = ptrTo(true)
		}, flow.CodeV5, flow.SeverityError},
		{"V6 join with one incoming edge", "parallel-join", func(d *flow.Document) {
			dropEdge(d, "e5")
		}, flow.CodeV6, flow.SeverityError},
		{"V7 boundary on a non-activity", "waits-and-boundary", func(d *flow.Document) {
			node(d, "too-slow").AttachedTo = "pause"
		}, flow.CodeV7, flow.SeverityError},
		{"V12 duplicate node name", "linear", func(d *flow.Document) {
			node(d, "done").Name = "Fetch"
		}, flow.CodeV12, flow.SeverityError},
		{"start event with an incoming edge", "linear", func(d *flow.Document) {
			d.Edges = append(d.Edges, flow.Edge{ID: "back", From: flow.Endpoint{Node: "fetch", Port: "main"}, To: "start", Kind: flow.EdgeSequence})
		}, flow.CodeElement, flow.SeverityError},
		{"end event with an outgoing edge", "linear", func(d *flow.Document) {
			d.Edges = append(d.Edges, flow.Edge{ID: "more", From: flow.Endpoint{Node: "done", Port: "main"}, To: "fetch", Kind: flow.EdgeSequence})
		}, flow.CodeElement, flow.SeverityError},
	})
}

func TestValidateDataAccessRules(t *testing.T) {
	t.Parallel()
	runRuleCases(t, []ruleCase{
		{"V8 unknown node name", "linear", func(d *flow.Document) {
			node(d, "done").Parameters = params("={{ $('Nope').item.json.x }}")
		}, flow.CodeV8, flow.SeverityError},
		{"V8 node reads itself", "linear", func(d *flow.Document) {
			node(d, "fetch").Parameters = params("={{ $('Fetch').item.json.x }}")
		}, flow.CodeV8, flow.SeverityError},
		{"V8 read of a sibling branch", "parallel-join", func(d *flow.Document) {
			node(d, "b").Parameters = params("={{ $('Branch A').item.json.x }}")
		}, flow.CodeV8, flow.SeverityError},
		{"V8 read of a downstream node", "linear", func(d *flow.Document) {
			node(d, "fetch").Parameters = params(`={{ $node["Done"].json.x }}`)
		}, flow.CodeV8, flow.SeverityError},
		{"V8 read of a node merged without a join", "exclusive-and-inclusive", func(d *flow.Document) {
			node(d, "after").Join = nil
		}, flow.CodeV8, flow.SeverityError},
		{"V8 read of a maybe-skipped branch without isExecuted", "exclusive-and-inclusive", func(d *flow.Document) {
			node(d, "after").Parameters = params("={{ $('Mail').item.json.x }}")
		}, flow.CodeV8, flow.SeverityWarning},
		{"V8 edge condition reads an unrelated node", "exclusive-and-inclusive", func(d *flow.Document) {
			edge(d, "e2").Condition = "$('After notifications').item.json.ok"
		}, flow.CodeV8, flow.SeverityError},
	})
}

func TestValidateDataAccessAccepts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		base   string
		mutate func(d *flow.Document)
	}{
		{"guarded read of a maybe-skipped branch", "exclusive-and-inclusive", func(d *flow.Document) {
			node(d, "after").Parameters = params("={{ $('Mail').isExecuted && $('Mail').item.json.x }}")
		}},
		{"a read inside a string literal outside braces is not a reference", "linear", func(d *flow.Document) {
			node(d, "done").Parameters = params("=see $('Nope') later")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := load(t, tt.base)
			tt.mutate(&doc)
			if r := flow.Validate(&doc); r.HasErrors() {
				t.Errorf("Validate = errors %v, want none", r.Errors())
			}
		})
	}
}

func TestValidateCallRules(t *testing.T) {
	t.Parallel()
	runRuleCases(t, []ruleCase{
		{"V11 call activity without a call block", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "child").Call = nil
		}, flow.CodeV11, flow.SeverityError},
		{"V11 flow calls itself without marking recursion", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "child").Call.Target = string(d.ID)
		}, flow.CodeV11, flow.SeverityError},
		{"V11 allowed target is the flow itself", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "dynamic").Call.AllowedTargets = []flow.ID{d.ID}
		}, flow.CodeV11, flow.SeverityError},
		{"V11 dynamic target without allowedTargets is a warning", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "dynamic").Call.AllowedTargets = nil
		}, flow.CodeV11, flow.SeverityWarning},
	})
}

func TestValidateIntentionalRecursionIsAccepted(t *testing.T) {
	t.Parallel()
	doc := load(t, "effects-calls-and-settings")
	node(&doc, "child").Call.Target = string(doc.ID)
	node(&doc, "child").Call.IntentionalRecursion = &flow.IntentionalRecursion{MaxDepth: 3}
	if r := flow.Validate(&doc); r.HasErrors() {
		t.Errorf("Validate with intentional recursion has errors: %v", r.Errors())
	}
}

func TestValidateReferenceRules(t *testing.T) {
	t.Parallel()
	runRuleCases(t, []ruleCase{
		{"edge to an unknown node", "linear", func(d *flow.Document) {
			edge(d, "e2").To = "ghost"
		}, flow.CodeReference, flow.SeverityError},
		{"edge from an unknown node", "linear", func(d *flow.Document) {
			edge(d, "e2").From.Node = "ghost"
		}, flow.CodeReference, flow.SeverityError},
		{"duplicate node id", "linear", func(d *flow.Document) {
			node(d, "done").ID = "fetch"
		}, flow.CodeReference, flow.SeverityError},
		{"duplicate edge id", "linear", func(d *flow.Document) {
			edge(d, "e2").ID = "e1"
		}, flow.CodeReference, flow.SeverityError},
		{"duplicate variable", "loop", func(d *flow.Document) {
			d.Variables = append(d.Variables, d.Variables[0])
		}, flow.CodeReference, flow.SeverityError},
		{"loop carries an undeclared variable", "loop", func(d *flow.Document) {
			node(d, "loop").Loop.Carry = []string{"nope"}
		}, flow.CodeReference, flow.SeverityError},
		{"loop carries a variable that is not carried", "loop", func(d *flow.Document) {
			node(d, "loop").Loop.Carry = []string{"pages"}
		}, flow.CodeReference, flow.SeverityError},
		{"boundary attached to an unknown node", "waits-and-boundary", func(d *flow.Document) {
			node(d, "too-slow").AttachedTo = "ghost"
		}, flow.CodeReference, flow.SeverityError},
		{"compensation handler unknown", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "charge").Compensation.Handler = "ghost"
		}, flow.CodeReference, flow.SeverityError},
		{"compensation handler is itself compensated", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "refund").Compensation = &flow.Compensation{Handler: "charge"}
		}, flow.CodeReference, flow.SeverityError},
	})
}

func TestValidateValueAndBlockRules(t *testing.T) {
	t.Parallel()
	runRuleCases(t, []ruleCase{
		{"wrong format tag", "linear", func(d *flow.Document) { d.Format = "orcon/flow@2" }, flow.CodeFormat, flow.SeverityError},
		{"flow id with a space", "linear", func(d *flow.Document) { d.ID = "bad id" }, flow.CodeValue, flow.SeverityError},
		{"empty flow name", "linear", func(d *flow.Document) { d.Name = "" }, flow.CodeValue, flow.SeverityError},
		{"node id with a slash", "linear", func(d *flow.Document) { node(d, "fetch").ID = "a/b" }, flow.CodeValue, flow.SeverityError},
		{"node type without a major", "linear", func(d *flow.Document) { node(d, "fetch").Type = "orcon.http" }, flow.CodeValue, flow.SeverityError},
		{"unknown element", "linear", func(d *flow.Document) { node(d, "fetch").Element = "subProcess" }, flow.CodeEnum, flow.SeverityError},
		{"port with a space", "linear", func(d *flow.Document) { edge(d, "e1").From.Port = "my port" }, flow.CodeValue, flow.SeverityError},
		{"negative edge order", "effects-calls-and-settings", func(d *flow.Document) { edge(d, "e1").Order = -1 }, flow.CodeValue, flow.SeverityError},
		{"parameters not an object", "linear", func(d *flow.Document) { node(d, "fetch").Parameters = jsonvalue.String("x") }, flow.CodeStructure, flow.SeverityError},
		{"extension key without namespace", "effects-calls-and-settings", func(d *flow.Document) {
			d.Ext = obj(jsonvalue.Member{Key: "plain", Value: jsonvalue.Null()})
		}, flow.CodeExt, flow.SeverityError},
		{"extension key in the reserved namespace", "effects-calls-and-settings", func(d *flow.Document) {
			d.Ext = obj(jsonvalue.Member{Key: "orcon.mine", Value: jsonvalue.Null()})
		}, flow.CodeExt, flow.SeverityError},
		{"invalid duration", "effects-calls-and-settings", func(d *flow.Document) { d.Settings.Timeouts.ExecutionActive = "30 minutes" }, flow.CodeValue, flow.SeverityError},
		{"month durations are not allowed", "effects-calls-and-settings", func(d *flow.Document) { d.Settings.Timeouts.ExecutionActive = "P1M" }, flow.CodeValue, flow.SeverityError},
		{"empty duration designator", "effects-calls-and-settings", func(d *flow.Document) { d.Settings.Timeouts.ExecutionActive = "PT" }, flow.CodeValue, flow.SeverityError},
		{"negative memory limit", "effects-calls-and-settings", func(d *flow.Document) { d.Settings.MemoryLimit = -1 }, flow.CodeValue, flow.SeverityError},
		{"retry factor below one", "effects-calls-and-settings", func(d *flow.Document) { node(d, "charge").Retry.Factor = 0.5 }, flow.CodeValue, flow.SeverityError},
		{"retry without attempts", "effects-calls-and-settings", func(d *flow.Document) { node(d, "charge").Retry.MaxAttempts = 0 }, flow.CodeValue, flow.SeverityError},
		{"concurrency zero", "effects-calls-and-settings", func(d *flow.Document) { node(d, "charge").Concurrency = &flow.IntOrKeyword{} }, flow.CodeValue, flow.SeverityError},
		{"concurrency number and keyword", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "charge").Concurrency = &flow.IntOrKeyword{N: 2, Keyword: "ALL"}
		}, flow.CodeValue, flow.SeverityError},
		{"multi-instance batch size zero", "multi-instance", func(d *flow.Document) { node(d, "each").MultiInstance.BatchSize = 0 }, flow.CodeValue, flow.SeverityError},
		{"join MATCH_FIELD without a field", "item-routing", func(d *flow.Document) {
			node(d, "combine").Join.Data = flow.JoinDataMatchField
			node(d, "combine").Join.MatchField = ""
		}, flow.CodeValue, flow.SeverityError},
		{"join MATCH_FIELD is not runnable yet", "item-routing", func(d *flow.Document) {
			node(d, "combine").Join.Data = flow.JoinDataMatchField
			node(d, "combine").Join.MatchField = "id"
		}, flow.CodeJoinData, flow.SeverityError},
		{"join MATCH_POSITION is not runnable yet", "item-routing", func(d *flow.Document) { node(d, "combine").Join.Data = flow.JoinDataMatchPosition }, flow.CodeJoinData, flow.SeverityError},
		{"join field without MATCH_FIELD", "parallel-join", func(d *flow.Document) { node(d, "join").Join.MatchField = "id" }, flow.CodeValue, flow.SeverityError},
		{"carry on an execution variable", "loop", func(d *flow.Document) { d.Variables[1].Carry = ptrTo(true) }, flow.CodeValue, flow.SeverityError},
		{"bad variable name", "loop", func(d *flow.Document) { d.Variables[0].Name = "1total" }, flow.CodeValue, flow.SeverityError},
		{"routing on a plain task", "linear", func(d *flow.Document) { node(d, "fetch").Routing = flow.RoutingExclusive }, flow.CodeBlock, flow.SeverityError},
		{"loop block on a plain task", "linear", func(d *flow.Document) { node(d, "fetch").Loop = &flow.Loop{MaxIterations: 1} }, flow.CodeBlock, flow.SeverityError},
		{"call block on a plain task", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "lookup").Call = &flow.Call{Target: "x", Version: flow.IntOrKeyword{Keyword: "CURRENT"}, Mode: flow.CallWait}
		}, flow.CodeBlock, flow.SeverityError},
		{"wait block on a plain task", "linear", func(d *flow.Document) {
			node(d, "fetch").Wait = &flow.Wait{Kind: flow.WaitTimer, Timer: &flow.Timer{Duration: "PT1S"}}
		}, flow.CodeBlock, flow.SeverityError},
		{"boundary fields on a plain task", "linear", func(d *flow.Document) { node(d, "fetch").AttachedTo = "start" }, flow.CodeBlock, flow.SeverityError},
		{"TIMER wait without a timer", "waits-and-boundary", func(d *flow.Document) { node(d, "pause").Wait.Timer = nil }, flow.CodeValue, flow.SeverityError},
		{"timer on a SIGNAL wait", "waits-and-boundary", func(d *flow.Document) {
			node(d, "callback").Wait.Timer = &flow.Timer{Duration: "PT1S"}
		}, flow.CodeBlock, flow.SeverityError},
		{"cycle on a wait timer", "waits-and-boundary", func(d *flow.Document) {
			node(d, "pause").Wait.Timer = &flow.Timer{Cycle: "R/PT1S"}
		}, flow.CodeValue, flow.SeverityError},
		{"timer with a duration and a date", "waits-and-boundary", func(d *flow.Document) {
			node(d, "pause").Wait.Timer.Date = "2026-10-12T10:00:00Z"
		}, flow.CodeValue, flow.SeverityError},
		{"timer date that is not RFC 3339", "waits-and-boundary", func(d *flow.Document) {
			node(d, "pause").Wait.Timer = &flow.Timer{Date: "next tuesday"}
		}, flow.CodeValue, flow.SeverityError},
		{"bad boundary cycle", "waits-and-boundary", func(d *flow.Document) {
			node(d, "reminder").EventDefinition.Timer = &flow.Timer{Cycle: "every 10s"}
		}, flow.CodeValue, flow.SeverityError},
		{"boundary without cancelActivity", "waits-and-boundary", func(d *flow.Document) { node(d, "too-slow").CancelActivity = nil }, flow.CodeValue, flow.SeverityError},
		{"boundary without eventDefinition", "waits-and-boundary", func(d *flow.Document) { node(d, "too-slow").EventDefinition = nil }, flow.CodeValue, flow.SeverityError},
		{"boundary with two event definitions", "waits-and-boundary", func(d *flow.Document) {
			node(d, "too-slow").EventDefinition.Compensation = true
		}, flow.CodeValue, flow.SeverityError},
		{"boundary without attachedTo", "waits-and-boundary", func(d *flow.Document) { node(d, "too-slow").AttachedTo = "" }, flow.CodeReference, flow.SeverityError},
		{"call without a version", "effects-calls-and-settings", func(d *flow.Document) { node(d, "child").Call.Version = flow.IntOrKeyword{} }, flow.CodeValue, flow.SeverityError},
		{"call version keyword other than CURRENT", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "child").Call.Version = flow.IntOrKeyword{Keyword: "LATEST"}
		}, flow.CodeEnum, flow.SeverityError},
		{"call target empty", "effects-calls-and-settings", func(d *flow.Document) { node(d, "child").Call.Target = "" }, flow.CodeValue, flow.SeverityError},
		{"call target with a space", "effects-calls-and-settings", func(d *flow.Document) { node(d, "child").Call.Target = "a b" }, flow.CodeValue, flow.SeverityError},
		{"intentional recursion depth zero", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "child").Call.IntentionalRecursion = &flow.IntentionalRecursion{}
		}, flow.CodeValue, flow.SeverityError},
	})
}

// TestEnumValuesUpperSnake covers the UPPER_SNAKE half of E-FMT-1: every
// enumerated value of the document accepts only its UPPER_SNAKE spelling and
// rejects the lowercase one (ADR-0007 section 11).
func TestEnumValuesUpperSnake(t *testing.T) {
	t.Parallel()
	lower := strings.ToLower
	runRuleCases(t, []ruleCase{
		{"branchMode", "effects-calls-and-settings", func(d *flow.Document) {
			d.Settings.BranchMode = flow.BranchMode(lower(string(d.Settings.BranchMode)))
		}, flow.CodeEnum, flow.SeverityError},
		{"priority", "effects-calls-and-settings", func(d *flow.Document) {
			d.Settings.Priority = flow.Priority(lower(string(d.Settings.Priority)))
		}, flow.CodeEnum, flow.SeverityError},
		{"retention", "effects-calls-and-settings", func(d *flow.Document) {
			d.Settings.Retention = flow.Retention(lower(string(d.Settings.Retention)))
		}, flow.CodeEnum, flow.SeverityError},
		{"retention failed_only", "loop", func(d *flow.Document) {
			d.Settings.Retention = "failedOnly"
		}, flow.CodeEnum, flow.SeverityError},
		{"routing", "item-routing", func(d *flow.Document) {
			n := node(d, "route")
			n.Routing = flow.Routing(lower(string(n.Routing)))
		}, flow.CodeEnum, flow.SeverityError},
		{"join mode", "parallel-join", func(d *flow.Document) {
			j := node(d, "join").Join
			j.Mode = flow.JoinMode(lower(string(j.Mode)))
		}, flow.CodeEnum, flow.SeverityError},
		{"join data", "parallel-join", func(d *flow.Document) {
			j := node(d, "join").Join
			j.Data = flow.JoinData(lower(string(j.Data)))
		}, flow.CodeEnum, flow.SeverityError},
		{"onError", "effects-calls-and-settings", func(d *flow.Document) {
			n := node(d, "charge")
			n.OnError = flow.OnError(lower(string(n.OnError)))
		}, flow.CodeEnum, flow.SeverityError},
		{"onError camelCase", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "charge").OnError = "stopExecution"
		}, flow.CodeEnum, flow.SeverityError},
		{"node concurrency ALL", "multi-instance", func(d *flow.Document) {
			node(d, "each").MultiInstance.Concurrency = &flow.IntOrKeyword{Keyword: "all"}
		}, flow.CodeEnum, flow.SeverityError},
		{"call version CURRENT", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "dynamic").Call.Version = flow.IntOrKeyword{Keyword: "current"}
		}, flow.CodeEnum, flow.SeverityError},
		{"call mode", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "dynamic").Call.Mode = "fire_and_forget"
		}, flow.CodeEnum, flow.SeverityError},
		{"wait kind", "waits-and-boundary", func(d *flow.Document) {
			node(d, "callback").Wait.Kind = "signal"
		}, flow.CodeEnum, flow.SeverityError},
		{"edge kind", "loop", func(d *flow.Document) {
			edge(d, "e4").Kind = "back"
		}, flow.CodeEnum, flow.SeverityError},
		{"attention notify", "effects-calls-and-settings", func(d *flow.Document) {
			node(d, "charge").Attention.Notify = "on"
		}, flow.CodeEnum, flow.SeverityError},
		{"variable scope", "loop", func(d *flow.Document) {
			d.Variables[0].Scope = "iteration"
		}, flow.CodeEnum, flow.SeverityError},
		{"variable accumulator", "loop", func(d *flow.Document) {
			d.Variables[1].Accumulator = "append"
		}, flow.CodeEnum, flow.SeverityError},
	})
	t.Run("hint names the UPPER_SNAKE spelling", func(t *testing.T) {
		t.Parallel()
		doc := load(t, "loop")
		doc.Settings.Retention = "failed_only"
		for _, issue := range flow.Validate(&doc).Errors() {
			if issue.Code == flow.CodeEnum && strings.Contains(issue.Message, `"FAILED_ONLY"`) {
				return
			}
		}
		t.Error("no ENUM issue suggesting \"FAILED_ONLY\"")
	})
}

// TestEnumValueSetsAreUpperSnake guards the value sets themselves: every
// enumerated value is non-empty UPPER_SNAKE, and elements keep BPMN names.
func TestEnumValueSetsAreUpperSnake(t *testing.T) {
	t.Parallel()
	var all []string
	add := func(vs ...string) { all = append(all, vs...) }
	for _, v := range flow.BranchModes() {
		add(string(v))
	}
	for _, v := range flow.Priorities() {
		add(string(v))
	}
	for _, v := range flow.Retentions() {
		add(string(v))
	}
	for _, v := range flow.Routings() {
		add(string(v))
	}
	for _, v := range flow.JoinModes() {
		add(string(v))
	}
	for _, v := range flow.JoinDataModes() {
		add(string(v))
	}
	for _, v := range flow.OnErrors() {
		add(string(v))
	}
	for _, v := range flow.CallModes() {
		add(string(v))
	}
	for _, v := range flow.WaitKinds() {
		add(string(v))
	}
	for _, v := range flow.EdgeKinds() {
		add(string(v))
	}
	for _, v := range flow.AttentionNotifies() {
		add(string(v))
	}
	for _, v := range flow.VarScopes() {
		add(string(v))
	}
	for _, v := range flow.Accumulators() {
		add(string(v))
	}
	for _, v := range flow.EffectKinds() {
		add(string(v))
	}
	for _, v := range flow.IdempotencyModes() {
		add(string(v))
	}
	for _, v := range flow.OnUncertains() {
		add(string(v))
	}
	for _, v := range flow.SafetyLevels() {
		add(string(v))
	}
	for _, v := range flow.KeyLocations() {
		add(string(v))
	}
	for _, v := range flow.Purities() {
		add(string(v))
	}
	add(flow.KeywordAll, flow.KeywordCurrent)
	for _, s := range all {
		if s == "" || s != strings.ToUpper(s) || strings.ContainsAny(s, " -.") {
			t.Errorf("enumerated value %q is not UPPER_SNAKE", s)
		}
	}
	for _, e := range flow.Elements() {
		if e == "" || e[0] < 'a' || e[0] > 'z' {
			t.Errorf("element %q should keep its BPMN camelCase name", e)
		}
	}
}

func TestValidateStopsAtTheFirstFailingLevel(t *testing.T) {
	t.Parallel()
	doc := load(t, "linear")
	node(&doc, "fetch").Element = "bogus"
	node(&doc, "done").Name = "Fetch" // V12 is a level-2 rule
	node(&doc, "start").Element = flow.ElementTask
	report := flow.Validate(&doc)
	if !has(report, flow.CodeEnum, flow.SeverityError) || !has(report, flow.CodeV12, flow.SeverityError) {
		t.Fatalf("want ENUM and V12 errors together, got %v", report.Issues)
	}
	if has(report, flow.CodeV1, flow.SeverityError) {
		t.Errorf("control-flow rules ran although earlier levels failed: %v", report.Issues)
	}
}

func TestValidateDoesNotModifyTheDocument(t *testing.T) {
	t.Parallel()
	for _, name := range conformanceNames(t) {
		doc := load(t, name)
		before, err := jsonvalue.Canonical(doc.Value())
		if err != nil {
			t.Fatal(err)
		}
		flow.Validate(&doc)
		after, err := jsonvalue.Canonical(doc.Value())
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(string(before), string(after)); diff != "" {
			t.Errorf("Validate(%s) changed the document (-before +after):\n%s", name, diff)
		}
	}
}

func TestReportHelpers(t *testing.T) {
	t.Parallel()
	doc := load(t, "linear")
	node(&doc, "done").Name = "Fetch"
	r := flow.Validate(&doc)
	err := r.Err()
	var ve *flow.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Err() = %v, want *ValidationError", err)
	}
	if !strings.Contains(err.Error(), "V12") {
		t.Errorf("Error() = %q, want it to name V12", err.Error())
	}
	if (flow.Report{}).Err() != nil {
		t.Error("Err() of an empty report is not nil")
	}
	if got := (&flow.ValidationError{}).Error(); got == "" {
		t.Error("Error() of an empty ValidationError is empty")
	}
	many := &flow.ValidationError{Report: flow.Report{Issues: []flow.Issue{
		{Severity: flow.SeverityError, Code: "A", Path: "/a", Message: "one"},
		{Severity: flow.SeverityError, Code: "B", Path: "/b", Message: "two"},
	}}}
	if !strings.Contains(many.Error(), "1 more") {
		t.Errorf("Error() = %q, want it to count the other errors", many.Error())
	}
	issue := flow.Issue{Severity: flow.SeverityWarning, Code: "V5", Path: "/nodes/1", Message: "m"}
	if got, want := issue.String(), "WARNING V5 at /nodes/1: m"; got != want {
		t.Errorf("Issue.String() = %q, want %q", got, want)
	}
}
