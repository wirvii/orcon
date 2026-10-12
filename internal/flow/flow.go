// Package flow holds the flow definition model: the versioned graph of nodes
// and edges that describes a workflow. It is a business-area package.
package flow

// Definition represents a versioned workflow graph.
type Definition struct {
	ID      string
	Version int
	Nodes   []Node
	Edges   []Edge
}

// Node is a single step of a flow graph.
type Node struct {
	ID   string
	Type string
}

// Edge connects two nodes of a flow graph.
type Edge struct {
	From string
	To   string
}
