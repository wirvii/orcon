// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import "github.com/wirvii/orcon/internal/platform/jsonvalue"

// Document is an orcon/flow@1 flow definition (ADR-0007 section 2).
//
// A field left at its zero value is absent from the document. The reader
// rejects empty strings and the writer omits zero values, so a document that
// passes [Validate] reads and writes back to the same JSON data. Booleans
// that may be explicitly false are pointers, so false and absent differ.
type Document struct {
	Format      string
	ID          ID
	Name        string
	Description string
	Settings    Settings
	Variables   []Variable
	Nodes       []Node
	Edges       []Edge
	// Editor holds the visual-only data (positions, sizes, sticky notes) as an
	// opaque object. It is excluded from the content hash.
	Editor jsonvalue.Value
	// Ext holds namespaced extension members ("vendor.key"); "orcon.*" is reserved.
	Ext jsonvalue.Value
}

// Settings are the execution-wide settings of a flow.
type Settings struct {
	BranchMode BranchMode
	Priority   Priority
	// Concurrency is the execution-level token limit (ADR-0014).
	Concurrency int
	Timeouts    *ExecutionTimeouts
	Retention   Retention
	// MemoryLimit is the per-execution memory cap in bytes (ADR-0016).
	MemoryLimit    int64
	ErrorFlow      ID
	CallDepthLimit int
	ImportedFrom   *ImportedFrom
}

// ExecutionTimeouts are the execution-level time limits.
type ExecutionTimeouts struct {
	// ExecutionActive is an ISO 8601 duration of active execution time.
	ExecutionActive string
}

// ImportedFrom records the n8n provenance of an imported flow.
type ImportedFrom struct {
	SourceVersion string
	ReportID      string
}

// Variable declares an execution or iteration variable (ADR-0008 section 4).
type Variable struct {
	Name        string
	Scope       VarScope
	Accumulator Accumulator
	// Carry initializes iteration i+1 with the final value of iteration i;
	// valid on iteration-scope variables only.
	Carry *bool
}

// Node is a step of a flow graph (ADR-0007 section 3).
type Node struct {
	ID      NodeID
	Name    string
	Element Element
	// Type is "<namespace>.<name>@<major>", for example "orcon.http@1".
	Type string
	// Parameters is an object; string values starting with "=" are expressions.
	Parameters jsonvalue.Value
	Disabled   *bool
	Ext        jsonvalue.Value

	Routing       Routing
	Join          *Join
	Loop          *Loop
	MultiInstance *MultiInstance
	Retry         *Retry
	Timeouts      *NodeTimeouts
	OnError       OnError
	Concurrency   *IntOrKeyword
	Compensation  *Compensation
	Call          *Call
	Wait          *Wait

	// AttachedTo, CancelActivity and EventDefinition describe a boundary event.
	AttachedTo      NodeID
	CancelActivity  *bool
	EventDefinition *EventDefinition

	Attention *Attention
}

// IntOrKeyword holds a field that is either a positive integer or one
// keyword (node concurrency: "ALL"; call version: "CURRENT").
type IntOrKeyword struct {
	N       int
	Keyword string
}

// Join is the synchronization rule of a node with several incoming edges.
type Join struct {
	Mode       JoinMode
	Data       JoinData
	MatchField string
	// Remaining applies to FIRST joins only; absent means CANCEL. An explicit
	// CANCEL is kept as written, so the content hash tells the two apart even
	// though they behave the same.
	Remaining JoinRemaining
}

// Loop configures an orcon.loop control node.
type Loop struct {
	MaxIterations int
	// Carry lists the iteration variables carried to the next iteration.
	Carry []string
}

// MultiInstance runs a node once per slice of its input.
type MultiInstance struct {
	Sequential          bool
	Concurrency         *IntOrKeyword
	InputCollection     string
	BatchSize           int
	CompletionCondition string
}

// Retry is the retry policy of a node (ADR-0011).
type Retry struct {
	MaxAttempts       int
	InitialInterval   string
	Factor            float64
	MaxInterval       string
	RespectRetryAfter *bool
}

// NodeTimeouts are the time limits of one node, as ISO 8601 durations.
type NodeTimeouts struct {
	Attempt string
	Node    string
}

// Compensation names the activity that undoes a node's work (ADR-0011 section 7).
type Compensation struct {
	Handler      NodeID
	Precondition string
}

// Call configures a call activity (ADR-0010 section 7).
type Call struct {
	// Target is a flow id, or an expression when it starts with "=".
	Target         string
	AllowedTargets []ID
	// Version is the keyword CURRENT or a pinned version number.
	Version              IntOrKeyword
	Mode                 CallMode
	PerItem              bool
	IntentionalRecursion *IntentionalRecursion
}

// IntentionalRecursion marks a call edge that closes a cycle on purpose.
type IntentionalRecursion struct {
	MaxDepth int
}

// Wait configures a durable wait (ADR-0010 section 8).
type Wait struct {
	Kind    WaitKind
	Timer   *Timer
	Timeout string
	// Form is the JSON Schema of the data a person fills in, kept opaque.
	Form jsonvalue.Value
}

// Timer is a time trigger. Exactly one field is set: a duration, an absolute
// RFC 3339 date, or (boundary events only) a repeating ISO 8601 interval.
type Timer struct {
	Duration string
	Date     string
	Cycle    string
}

// EventDefinition is the trigger of a boundary event. Exactly one of the
// three is set.
type EventDefinition struct {
	Timer        *Timer
	Error        *ErrorDefinition
	Compensation bool
}

// ErrorDefinition matches the errors an error boundary event catches.
type ErrorDefinition struct {
	// Codes limits the match; nil catches every error. A present list is
	// never empty.
	Codes []string
}

// Attention overrides the human-attention notification class (ADR-0073 section 1a).
type Attention struct {
	Notify AttentionNotify
}

// Edge connects two nodes (ADR-0007 section 4).
type Edge struct {
	ID        EdgeID
	From      Endpoint
	To        NodeID
	Condition string
	Default   *bool
	Kind      EdgeKind
	// Order sequences siblings in SEQUENTIAL branch mode; it starts at 1 and
	// 0 means absent.
	Order int
}

// Endpoint is the source of an edge: a node and one of its output ports.
type Endpoint struct {
	Node NodeID
	Port string
}

// Port names the engine itself gives meaning to; other names are
// identifiers chosen by node types.
const (
	// PortMain is the default output port.
	PortMain = "main"
	// PortTimeout is the output port taken when a wait times out.
	PortTimeout = "timeout"
)

// isTrue reports whether an optional boolean is present and true.
func isTrue(b *bool) bool { return b != nil && *b }
