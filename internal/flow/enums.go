// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

// Every enumerated value of the document is UPPER_SNAKE (ADR-0007 section 11,
// ADR-0080 section 3). Identifiers keep their form: ids, names, node types,
// port names, BPMN element names, the format tag and expression text.

// BranchMode decides whether sibling tokens run at once or in edge order.
type BranchMode string

// Branch modes.
const (
	// BranchParallel makes sibling tokens runnable immediately (the default).
	BranchParallel BranchMode = "PARALLEL"
	// BranchSequential runs siblings one after another in edge order.
	BranchSequential BranchMode = "SEQUENTIAL"
)

// BranchModes returns every valid [BranchMode].
func BranchModes() []BranchMode { return []BranchMode{BranchParallel, BranchSequential} }

// Priority is the scheduling class of an execution (ADR-0014).
type Priority string

// Priorities.
const (
	// PriorityHigh is the highest scheduling class.
	PriorityHigh Priority = "HIGH"
	// PriorityNormal is the default scheduling class.
	PriorityNormal Priority = "NORMAL"
	// PriorityLow is the lowest scheduling class.
	PriorityLow Priority = "LOW"
)

// Priorities returns every valid [Priority].
func Priorities() []Priority { return []Priority{PriorityHigh, PriorityNormal, PriorityLow} }

// Retention is the data retention policy of a flow's executions (ADR-0017).
type Retention string

// Retention policies.
const (
	// RetentionAlways keeps execution data of every execution.
	RetentionAlways Retention = "ALWAYS"
	// RetentionFailedOnly keeps execution data of failed executions only.
	RetentionFailedOnly Retention = "FAILED_ONLY"
	// RetentionNever keeps no execution data once an execution ends.
	RetentionNever Retention = "NEVER"
	// RetentionDefault defers to the instance default.
	RetentionDefault Retention = "DEFAULT"
)

// Retentions returns every valid [Retention].
func Retentions() []Retention {
	return []Retention{RetentionAlways, RetentionFailedOnly, RetentionNever, RetentionDefault}
}

// Routing is how an item-routing node sends items to its ports (ADR-0010 section 4).
type Routing string

// Routing modes.
const (
	// RoutingExclusive sends each item to exactly one port.
	RoutingExclusive Routing = "EXCLUSIVE"
	// RoutingInclusive may send an item to several ports.
	RoutingInclusive Routing = "INCLUSIVE"
)

// Routings returns every valid [Routing].
func Routings() []Routing { return []Routing{RoutingExclusive, RoutingInclusive} }

// JoinMode is the firing rule of a join (ADR-0010 section 3).
type JoinMode string

// Join modes.
const (
	// JoinAll fires when every incoming edge completed with a live token.
	JoinAll JoinMode = "ALL"
	// JoinActivated fires when all edges completed and at least one was live.
	JoinActivated JoinMode = "ACTIVATED"
	// JoinFirst fires on the first live arrival and cancels the rest.
	JoinFirst JoinMode = "FIRST"
)

// JoinModes returns every valid [JoinMode].
func JoinModes() []JoinMode { return []JoinMode{JoinAll, JoinActivated, JoinFirst} }

// JoinData is how a join combines the data of its inputs.
type JoinData string

// Join data modes.
const (
	// JoinDataAppend concatenates the batches in incoming-edge order.
	JoinDataAppend JoinData = "APPEND"
	// JoinDataMatchField pairs items by the value of a field.
	JoinDataMatchField JoinData = "MATCH_FIELD"
	// JoinDataMatchPosition pairs items by position.
	JoinDataMatchPosition JoinData = "MATCH_POSITION"
)

// JoinDataModes returns every valid [JoinData].
func JoinDataModes() []JoinData {
	return []JoinData{JoinDataAppend, JoinDataMatchField, JoinDataMatchPosition}
}

// OnError is the error routing of a node (ADR-0011).
type OnError string

// Error routings.
const (
	// OnErrorStopExecution fails the execution.
	OnErrorStopExecution OnError = "STOP_EXECUTION"
	// OnErrorContinueWithError continues with the error as output data.
	OnErrorContinueWithError OnError = "CONTINUE_WITH_ERROR"
	// OnErrorErrorOutput sends the error to the node's error port.
	OnErrorErrorOutput OnError = "ERROR_OUTPUT"
)

// OnErrors returns every valid [OnError].
func OnErrors() []OnError {
	return []OnError{OnErrorStopExecution, OnErrorContinueWithError, OnErrorErrorOutput}
}

// CallMode is how a call activity treats the child execution (ADR-0010 section 7).
type CallMode string

// Call modes.
const (
	// CallWait parks the parent token until the child ends.
	CallWait CallMode = "WAIT"
	// CallFireAndForget continues the parent with the child's execution id.
	CallFireAndForget CallMode = "FIRE_AND_FORGET"
)

// CallModes returns every valid [CallMode].
func CallModes() []CallMode { return []CallMode{CallWait, CallFireAndForget} }

// WaitKind is the kind of a durable wait (ADR-0010 section 8).
type WaitKind string

// Wait kinds.
const (
	// WaitTimer waits for a duration or a date.
	WaitTimer WaitKind = "TIMER"
	// WaitSignal waits for an external notice.
	WaitSignal WaitKind = "SIGNAL"
	// WaitHuman waits for a person to resolve a task.
	WaitHuman WaitKind = "HUMAN"
)

// WaitKinds returns every valid [WaitKind].
func WaitKinds() []WaitKind { return []WaitKind{WaitTimer, WaitSignal, WaitHuman} }

// EdgeKind tells forward edges from loop back edges (ADR-0007 section 4).
type EdgeKind string

// Edge kinds.
const (
	// EdgeSequence is an ordinary forward edge.
	EdgeSequence EdgeKind = "SEQUENCE"
	// EdgeBack returns to a loop control node.
	EdgeBack EdgeKind = "BACK"
)

// EdgeKinds returns every valid [EdgeKind].
func EdgeKinds() []EdgeKind { return []EdgeKind{EdgeSequence, EdgeBack} }

// AttentionNotify overrides the human-attention notification class of a wait
// kind (ADR-0073 section 1a).
type AttentionNotify string

// Attention notification settings.
const (
	// AttentionDefault follows the default of the wait kind.
	AttentionDefault AttentionNotify = "DEFAULT"
	// AttentionOn forces a notification.
	AttentionOn AttentionNotify = "ON"
	// AttentionOff suppresses the notification; an error on mandatory kinds.
	AttentionOff AttentionNotify = "OFF"
)

// AttentionNotifies returns every valid [AttentionNotify].
func AttentionNotifies() []AttentionNotify {
	return []AttentionNotify{AttentionDefault, AttentionOn, AttentionOff}
}

// VarScope is the lifetime of a variable (ADR-0008 section 4).
type VarScope string

// Variable scopes.
const (
	// ScopeExecution lives as long as the execution.
	ScopeExecution VarScope = "EXECUTION"
	// ScopeIteration lives for one loop iteration, optionally carried on.
	ScopeIteration VarScope = "ITERATION"
)

// VarScopes returns every valid [VarScope].
func VarScopes() []VarScope { return []VarScope{ScopeExecution, ScopeIteration} }

// Accumulator is the delta operation of an accumulator variable.
type Accumulator string

// Accumulators.
const (
	// AccumulatorSum adds deltas.
	AccumulatorSum Accumulator = "SUM"
	// AccumulatorAppend appends deltas in commit order.
	AccumulatorAppend Accumulator = "APPEND"
	// AccumulatorMax keeps the largest delta.
	AccumulatorMax Accumulator = "MAX"
	// AccumulatorMin keeps the smallest delta.
	AccumulatorMin Accumulator = "MIN"
)

// Accumulators returns every valid [Accumulator].
func Accumulators() []Accumulator {
	return []Accumulator{AccumulatorSum, AccumulatorAppend, AccumulatorMax, AccumulatorMin}
}

// EffectKind is the external effect class of an operation (ADR-0012 section 1).
type EffectKind string

// Effect kinds.
const (
	// EffectRead reads without side effects.
	EffectRead EffectKind = "READ"
	// EffectWrite creates or changes external state.
	EffectWrite EffectKind = "WRITE"
	// EffectDelete removes external state.
	EffectDelete EffectKind = "DELETE"
	// EffectUnknown is treated as a write.
	EffectUnknown EffectKind = "UNKNOWN"
)

// EffectKinds returns every valid [EffectKind].
func EffectKinds() []EffectKind {
	return []EffectKind{EffectRead, EffectWrite, EffectDelete, EffectUnknown}
}

// IdempotencyMode is how an operation deduplicates repeated requests.
type IdempotencyMode string

// Idempotency modes.
const (
	// IdempotencyProviderKey sends a key the provider deduplicates on.
	IdempotencyProviderKey IdempotencyMode = "PROVIDER_KEY"
	// IdempotencyNaturalKey relies on a natural key of the data.
	IdempotencyNaturalKey IdempotencyMode = "NATURAL_KEY"
	// IdempotencyNone offers no deduplication.
	IdempotencyNone IdempotencyMode = "NONE"
)

// IdempotencyModes returns every valid [IdempotencyMode].
func IdempotencyModes() []IdempotencyMode {
	return []IdempotencyMode{IdempotencyProviderKey, IdempotencyNaturalKey, IdempotencyNone}
}

// OnUncertain is the recovery of an operation whose outcome is unknown.
type OnUncertain string

// Recovery strategies for an uncertain outcome.
const (
	// OnUncertainResendWithKey sends the request again with the same key.
	OnUncertainResendWithKey OnUncertain = "RESEND_WITH_KEY"
	// OnUncertainReconcile asks the provider what happened.
	OnUncertainReconcile OnUncertain = "RECONCILE"
	// OnUncertainSuspend parks the token for a person.
	OnUncertainSuspend OnUncertain = "SUSPEND"
)

// OnUncertains returns every valid [OnUncertain].
func OnUncertains() []OnUncertain {
	return []OnUncertain{OnUncertainResendWithKey, OnUncertainReconcile, OnUncertainSuspend}
}

// Element is the BPMN 2.0 element class of a node. The names are fixed by
// BPMN and keep their camelCase form.
type Element string

// BPMN element classes (ADR-0007 section 3).
const (
	// ElementStartEvent starts a flow.
	ElementStartEvent Element = "startEvent"
	// ElementEndEvent ends a path of a flow.
	ElementEndEvent Element = "endEvent"
	// ElementTask is a generic activity.
	ElementTask Element = "task"
	// ElementServiceTask is an activity calling a service.
	ElementServiceTask Element = "serviceTask"
	// ElementScriptTask is an activity running a script.
	ElementScriptTask Element = "scriptTask"
	// ElementUserTask is an activity completed by a person.
	ElementUserTask Element = "userTask"
	// ElementReceiveTask is an activity waiting for a message.
	ElementReceiveTask Element = "receiveTask"
	// ElementSendTask is an activity sending a message.
	ElementSendTask Element = "sendTask"
	// ElementCallActivity calls another flow.
	ElementCallActivity Element = "callActivity"
	// ElementExclusiveGateway chooses one path.
	ElementExclusiveGateway Element = "exclusiveGateway"
	// ElementInclusiveGateway chooses any number of paths.
	ElementInclusiveGateway Element = "inclusiveGateway"
	// ElementParallelGateway takes every path.
	ElementParallelGateway Element = "parallelGateway"
	// ElementEventBasedGateway waits for the first of several events.
	ElementEventBasedGateway Element = "eventBasedGateway"
	// ElementComplexGateway has an explicit activation rule.
	ElementComplexGateway Element = "complexGateway"
	// ElementIntermediateCatchEvent waits for an event mid-flow.
	ElementIntermediateCatchEvent Element = "intermediateCatchEvent"
	// ElementIntermediateThrowEvent raises an event mid-flow.
	ElementIntermediateThrowEvent Element = "intermediateThrowEvent"
	// ElementBoundaryEvent is attached to an activity.
	ElementBoundaryEvent Element = "boundaryEvent"
)

// Elements returns every valid [Element].
func Elements() []Element {
	return []Element{
		ElementStartEvent, ElementEndEvent, ElementTask, ElementServiceTask, ElementScriptTask,
		ElementUserTask, ElementReceiveTask, ElementSendTask, ElementCallActivity,
		ElementExclusiveGateway, ElementInclusiveGateway, ElementParallelGateway,
		ElementEventBasedGateway, ElementComplexGateway, ElementIntermediateCatchEvent,
		ElementIntermediateThrowEvent, ElementBoundaryEvent,
	}
}

// isActivity reports whether the element is a BPMN activity, the only kind
// of node a boundary event may be attached to (V7).
func (e Element) isActivity() bool {
	switch e {
	case ElementTask, ElementServiceTask, ElementScriptTask, ElementUserTask,
		ElementReceiveTask, ElementSendTask, ElementCallActivity:
		return true
	case ElementStartEvent, ElementEndEvent, ElementExclusiveGateway, ElementInclusiveGateway,
		ElementParallelGateway, ElementEventBasedGateway, ElementComplexGateway,
		ElementIntermediateCatchEvent, ElementIntermediateThrowEvent, ElementBoundaryEvent:
		return false
	default:
		return false
	}
}

// Keywords that stand in for a number in the fields that accept both.
const (
	// KeywordAll is the node concurrency "no node-level cap".
	KeywordAll = "ALL"
	// KeywordCurrent is the call version "active published version".
	KeywordCurrent = "CURRENT"
)
