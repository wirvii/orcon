package flow

// Definition represents a versioned workflow graph.
type Definition struct {
 ID string
 Version int
 Nodes []Node
 Edges []Edge
}

type Node struct { ID string; Type string }
type Edge struct { From string; To string }
