// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import (
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Item is one item flowing between nodes: its JSON data and its binary
// metadata (ADR-0008 section 1).
type Item struct {
	JSON jsonvalue.Value
	// Binary holds the metadata of the item's binary properties; the zero
	// value means the item has none.
	Binary jsonvalue.Value
}

// NodeOutput is what an expression can read of another node, $('Name')
// (ADR-0008 section 4, rule 2).
type NodeOutput struct {
	// Executed reports whether the node ran. Reading the items of a node that
	// did not run is an error; $('Name').isExecuted is the explicit test.
	Executed bool
	// Items are the items of the node's main output.
	Items []Item
	// Params are the parameters of the node as the author wrote them.
	Params jsonvalue.Value
	// Paired returns the item of this node that the item at index of the
	// current activation descends from, following lineage (ADR-0008). It is
	// optional; without it $('Name').item is an error.
	Paired func(index int) (Item, bool)
}

// NodeSource gives access to the outputs of other nodes by name. The
// interpreter implements it over the execution data; a read of a node that
// does not exist reports false.
type NodeSource interface {
	Node(name string) (NodeOutput, bool)
}

// ExecutionInfo is the read-only execution context ($execution).
type ExecutionInfo struct {
	ID string
	// Mode is "test" or "production", the values n8n documents.
	Mode          string
	ResumeURL     string
	ResumeFormURL string
	// Variables are the execution variables an expression may read as
	// $execution.variables.<name> (ADR-0008 section 4.3): an object from name
	// to value, constant during the activation. Zero means none has a value;
	// reading one then gives undefined.
	Variables jsonvalue.Value
}

// WorkflowInfo describes the flow being executed ($workflow).
type WorkflowInfo struct {
	ID     string
	Name   string
	Active bool
}

// PrevNode describes the node that produced the input ($prevNode).
type PrevNode struct {
	Name        string
	OutputIndex int
	RunIndex    int
}

// Env is everything an expression can see while it is evaluated for one item.
// The interpreter fills one per item (or reuses one and changes ItemIndex).
type Env struct {
	// Input are the input items of the activation ($input.all()). The current
	// item is Input[ItemIndex]; $json is its JSON.
	Input     []Item
	ItemIndex int
	// RunIndex is the iteration of the enclosing loop scope ($runIndex).
	RunIndex int
	// Nodes resolves $('Name'). Nil means no other node is readable.
	Nodes NodeSource
	// Activation holds the recorded time and random stream. It is required by
	// every expression that runs in the sandbox.
	Activation *Activation
	// Timezone is the IANA zone of the flow ($now, $today and Luxon default
	// zone). Empty means UTC.
	Timezone  string
	Execution ExecutionInfo
	Workflow  WorkflowInfo
	PrevNode  PrevNode
	// Vars are the non-secret project variables of the environment ($vars,
	// ADR-0057). Zero means none.
	Vars jsonvalue.Value
	// Parameters are the parameters of the node being evaluated ($parameter).
	Parameters jsonvalue.Value
	// Identity names the organization, execution and node of the evaluation.
	// The sandbox adapter logs it when it abandons the call; zero is allowed.
	Identity CallIdentity
}

// current returns the current item, or an item with empty data when the
// activation has no input.
func (e *Env) current() Item {
	if e.ItemIndex >= 0 && e.ItemIndex < len(e.Input) {
		return e.Input[e.ItemIndex]
	}
	return Item{}
}

// Result is the value of an evaluation.
type Result struct {
	// Value is the JSON value. It is null when Undefined is set.
	Value jsonvalue.Value
	// Undefined reports that the expression evaluated to JavaScript undefined,
	// which JSON cannot hold. The interpreter treats it as "no value".
	Undefined bool
}
