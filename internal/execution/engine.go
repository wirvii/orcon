package execution

import "context"

// Engine is the contract for a durable workflow execution engine.
// Implementation intentionally deferred until state semantics are specified.
type Engine interface {
 Start(ctx context.Context, flowID string, input []byte) (executionID string, err error)
}
