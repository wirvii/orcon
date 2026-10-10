package workflow

// Definition represents a versioned workflow graph.
type Definition struct {
 ID string `json:"id"`
 Version int `json:"version"`
 Nodes []Node `json:"nodes"`
 Edges []Edge `json:"edges"`
}

type Node struct { ID string `json:"id"`; Type string `json:"type"` }
type Edge struct { From string `json:"from"`; To string `json:"to"` }
