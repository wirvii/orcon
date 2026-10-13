// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/flow"
)

func TestCompileRejectsAnInvalidFlow(t *testing.T) {
	t.Parallel()
	if _, err := execution.Compile(nil); !errors.Is(err, execution.ErrInvalidFlow) {
		t.Errorf("Compile(nil) = %v, want ErrInvalidFlow", err)
	}
	doc := linearFlow().Doc()
	doc.Format = "orcon/flow@9"
	_, err := execution.Compile(&doc)
	var ve *flow.ValidationError
	if !errors.Is(err, execution.ErrInvalidFlow) || !errors.As(err, &ve) {
		t.Errorf("Compile(bad format) = %v, want ErrInvalidFlow wrapping the validation error", err)
	}
}

func TestCompileExposesTheFlowID(t *testing.T) {
	t.Parallel()
	if got := executiontest.MustCompile(t, linearFlow()).FlowID(); got != executiontest.FlowID {
		t.Errorf("FlowID() = %q, want %q", got, executiontest.FlowID)
	}
}

func ptrTo[T any](v T) *T { return &v }

func TestCompileNamesWhatIsNotSupportedYet(t *testing.T) {
	t.Parallel()
	timer := &flow.Wait{Kind: flow.WaitTimer, Timer: &flow.Timer{Duration: "PT1S"}}
	tests := []struct {
		name    string
		build   func() *executiontest.Builder
		feature string
		cut     string
	}{
		{
			"multi-instance", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.MultiInstance = &flow.MultiInstance{BatchSize: 1} })
			}, "multiInstance", "C9",
		},
		{
			"wait on a receive task", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.Element = flow.ElementReceiveTask; n.Wait = timer })
			}, "wait", "C8",
		},
		{
			"user task", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.Element = flow.ElementUserTask })
			}, "element userTask", "C8",
		},
		{
			"intermediate catch event", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.Element = flow.ElementIntermediateCatchEvent; n.Wait = timer })
			}, "element intermediateCatchEvent", "C8",
		},
		{
			"intermediate throw event", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.Element = flow.ElementIntermediateThrowEvent })
			}, "element intermediateThrowEvent", "C8",
		},
		{
			"event based gateway", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.Element = flow.ElementEventBasedGateway })
			}, "element eventBasedGateway", "C8",
		},
		{
			"complex gateway", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.Element = flow.ElementComplexGateway })
			}, "element complexGateway", "none",
		},
		{
			"call activity", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) {
					n.Element = flow.ElementCallActivity
					n.Call = &flow.Call{
						Target: "01930000-0000-7000-8000-0000000000bb", Version: flow.IntOrKeyword{Keyword: "CURRENT"}, Mode: flow.CallWait,
					}
				})
			}, "call", "C9",
		},
		{
			"boundary event", func() *executiontest.Builder {
				b := linearFlow()
				b.Task("bound")
				return b.Modify("bound", func(n *flow.Node) {
					n.Element = flow.ElementBoundaryEvent
					n.AttachedTo = "a"
					n.CancelActivity = ptrTo(true)
					n.EventDefinition = &flow.EventDefinition{Timer: &flow.Timer{Duration: "PT1S"}}
				})
			}, "boundary event definition", "C8",
		},
		{
			"compensation", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.Compensation = &flow.Compensation{Handler: "b"} })
			}, "compensation", "C7",
		},
		{
			"retry", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.Retry = &flow.Retry{MaxAttempts: 2} })
			}, "retry", "C7",
		},
		{
			"node timeouts", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.Timeouts = &flow.NodeTimeouts{Attempt: "PT5S"} })
			}, "timeouts", "C7",
		},
		{
			"error output", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.OnError = flow.OnErrorErrorOutput })
			}, "onError ERROR_OUTPUT", "C7",
		},
		{
			"continue with error", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.OnError = flow.OnErrorContinueWithError })
			}, "onError CONTINUE_WITH_ERROR", "C7",
		},
		{
			"disabled node", func() *executiontest.Builder {
				return linearFlow().Modify("a", func(n *flow.Node) { n.Disabled = ptrTo(true) })
			}, "disabled", "none",
		},
		{
			"loop carry", func() *executiontest.Builder {
				b := counterLoop(3).IterationVariable("total")
				b.Modify("L", func(n *flow.Node) { n.Loop.Carry = []string{"total"} })
				return b
			}, "loop.carry", "C10",
		},
		{
			"condition on a task edge", func() *executiontest.Builder {
				return executiontest.NewFlow("cond").Start("s").Task("a").End("e").
					Edge("s", "a", executiontest.Cond("c")).Edge("a", "e")
			}, "a condition on an edge", "none",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := tt.build().Doc()
			_, err := execution.Compile(&doc)
			if errors.Is(err, execution.ErrInvalidFlow) {
				t.Fatalf("Compile() = %v: the test document must be a valid flow", err)
			}
			var ue *execution.UnsupportedError
			if !errors.Is(err, execution.ErrUnsupported) || !errors.As(err, &ue) {
				t.Fatalf("Compile() = %v, want an UnsupportedError", err)
			}
			found := false
			for _, issue := range ue.Issues {
				found = found || (strings.Contains(issue.Feature, tt.feature) && strings.HasPrefix(issue.Cut, tt.cut))
			}
			if !found {
				t.Errorf("issues = %v, want one for %q owned by cut %s", ue.Issues, tt.feature, tt.cut)
			}
			if !strings.Contains(err.Error(), tt.feature) {
				t.Errorf("Error() = %q, want it to name %q", err.Error(), tt.feature)
			}
		})
	}
}

func forkJoin() *executiontest.Builder {
	return executiontest.NewFlow("fork-join").
		Start("s").Parallel("p").Task("a").Task("b").Parallel("j").End("e").
		Join("j", flow.JoinAll).
		Chain("s", "p").Edge("p", "a").Edge("p", "b").Edge("a", "j").Edge("b", "j").Edge("j", "e")
}

func TestCompileRejectsFlowLevelSettingsOfLaterCuts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(*flow.Document)
		feature string
	}{
		{"execution timeout", func(d *flow.Document) { d.Settings.Timeouts = &flow.ExecutionTimeouts{ExecutionActive: "PT1M"} }, "settings.timeouts"},
		{"error flow", func(d *flow.Document) { d.Settings.ErrorFlow = "01930000-0000-7000-8000-0000000000bb" }, "settings.errorFlow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := linearFlow().Doc()
			tt.mutate(&doc)
			_, err := execution.Compile(&doc)
			var ue *execution.UnsupportedError
			if !errors.As(err, &ue) || len(ue.Issues) != 1 || ue.Issues[0].Feature != tt.feature || ue.Issues[0].Node != "" {
				t.Errorf("Compile() = %v, want one flow-level issue for %s", err, tt.feature)
			}
		})
	}
}

func TestCompileRejectsLoopShapesTheScopeAccountingCannotRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		build func() *executiontest.Builder
		want  string
	}{
		{
			"a loop port other than loop", func() *executiontest.Builder {
				return executiontest.NewFlow("port").Start("s").Loop("L", 3).Task("work").If("more", flow.RoutingExclusive).End("e").
					Edge("s", "L").Edge("L", "work", executiontest.Port("body")).Edge("work", "more").
					Edge("more", "L", executiontest.Port("true"), executiontest.Back()).Edge("more", "e", executiontest.Port("false"))
			}, `port "body" on a loop control`,
		},
		{
			"two entry edges", func() *executiontest.Builder {
				return executiontest.NewFlow("entries").Start("s").Parallel("p").Task("a").Loop("L", 3).Task("work").If("more", flow.RoutingExclusive).End("e").
					Chain("s", "p").Edge("p", "a").Edge("p", "L").Edge("a", "L").
					Edge("L", "work", executiontest.Port("loop")).Edge("work", "more").
					Edge("more", "L", executiontest.Port("true"), executiontest.Back()).Edge("more", "e", executiontest.Port("false"))
			}, "2 incoming forward edges",
		},
		{
			"a back edge from outside the loop", func() *executiontest.Builder {
				return executiontest.NewFlow("outside").Start("s").Parallel("p").Loop("L", 3).Task("work").Task("other").End("e").End("e2").
					Chain("s", "p").Edge("p", "L").Edge("p", "other").
					Edge("L", "work", executiontest.Port("loop")).Edge("work", "e").
					Edge("other", "L", executiontest.Back()).Edge("other", "e2")
			}, "a back edge from a node its loop control does not reach",
		},
		{
			"an edge entering the body from outside", func() *executiontest.Builder {
				return executiontest.NewFlow("side").Start("s").Parallel("p").Loop("L", 3).Task("work").Task("side").If("more", flow.RoutingExclusive).End("e").
					Chain("s", "p").Edge("p", "L").Edge("p", "side").
					Edge("L", "work", executiontest.Port("loop")).Edge("work", "more").Edge("side", "more").
					Edge("more", "L", executiontest.Port("true"), executiontest.Back()).Edge("more", "e", executiontest.Port("false"))
			}, "entering the body of loop",
		},
		{
			"loops that overlap without nesting", func() *executiontest.Builder {
				// L1 body: a, b. L2 body: b, c. They share b but neither contains the other.
				return executiontest.NewFlow("overlap").Start("s").Parallel("p").Loop("L1", 3).Loop("L2", 3).
					Task("a").Task("b").Task("c").If("m1", flow.RoutingExclusive).If("m2", flow.RoutingExclusive).End("e").
					Chain("s", "p").Edge("p", "L1").Edge("p", "L2").
					Edge("L1", "a", executiontest.Port("loop")).Edge("L2", "c", executiontest.Port("loop")).
					Edge("a", "b").Edge("c", "b").Edge("b", "m1").Edge("b", "m2").
					Edge("m1", "L1", executiontest.Port("true"), executiontest.Back()).
					Edge("m2", "L2", executiontest.Port("true"), executiontest.Back()).
					Edge("m1", "e", executiontest.Port("false")).Edge("m2", "e", executiontest.Port("false"))
			}, "overlapping",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := tt.build().Doc()
			_, err := execution.Compile(&doc)
			if errors.Is(err, execution.ErrInvalidFlow) {
				t.Fatalf("Compile() = %v: the test document must be a valid flow", err)
			}
			if !errors.Is(err, execution.ErrUnsupported) || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Compile() = %v, want ErrUnsupported mentioning %q", err, tt.want)
			}
		})
	}
}

func TestUnsupportedIssueDescribesItsPlace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		issue execution.UnsupportedIssue
		want  string
	}{
		{execution.UnsupportedIssue{Node: "a", Feature: "retry", Cut: "C7"}, `node "a" uses retry (cut C7)`},
		{execution.UnsupportedIssue{Edge: "e1", Feature: "x", Cut: "C1"}, `edge "e1" uses x (cut C1)`},
		{execution.UnsupportedIssue{Feature: "settings.timeouts", Cut: "C7"}, "flow uses settings.timeouts (cut C7)"},
	}
	for _, tt := range tests {
		if got := tt.issue.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestCompileRejectsJoinDataModesTheEngineCannotRun(t *testing.T) {
	t.Parallel()
	for _, data := range []flow.JoinData{flow.JoinDataMatchField, flow.JoinDataMatchPosition} {
		t.Run(string(data), func(t *testing.T) {
			t.Parallel()
			doc := forkJoin().Modify("j", func(n *flow.Node) {
				n.Join.Data = data
				if data == flow.JoinDataMatchField {
					n.Join.MatchField = "id"
				}
			}).Doc()
			// The check lives in flow.Validate, shared with publication, so
			// Compile reports it as an invalid flow.
			report := flow.Validate(&doc)
			if !report.Has(flow.CodeJoinData, flow.SeverityError) {
				t.Fatalf("Validate() issues = %v, want a %s error", report.Issues, flow.CodeJoinData)
			}
			if _, err := execution.Compile(&doc); !errors.Is(err, execution.ErrInvalidFlow) {
				t.Errorf("Compile() = %v, want ErrInvalidFlow", err)
			}
		})
	}
}
