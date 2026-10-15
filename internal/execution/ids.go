// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"strconv"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/id"
)

// ID identifies one execution of a flow revision.
type ID string

// TokenID identifies a token. Token ids are deterministic (ADR-0008 section 6),
// so recomputing a step after a crash produces the same rows.
type TokenID string

// NodeExecutionID identifies one execution of a node by a token in a scope.
type NodeExecutionID string

// hashID is the function H of ADR-0008 section 6, written as UUID text.
func hashID(fields ...string) string { return id.Derive(fields...).String() }

// rootTokenID is H(executionId, "root").
func rootTokenID(exec ID) TokenID {
	return TokenID(hashID(string(exec), "root"))
}

// forkTokenID is H(parentTokenId, splitNodeId, scopePath, branchIndex).
func forkTokenID(parent TokenID, split flow.NodeID, scope ScopePath, branch int) TokenID {
	return TokenID(hashID(string(parent), string(split), string(scope), strconv.Itoa(branch)))
}

// joinTokenID is H(executionId, joinNodeId, scopePath), the continuation
// token of a join. It is unique per join instance, so two workers that both
// observe the firing condition create the same row.
func joinTokenID(exec ID, join flow.NodeID, scope ScopePath) TokenID {
	return TokenID(hashID(string(exec), string(join), string(scope)))
}

// roundTokenID is the continuation token of a round of a LET_FINISH join. It
// derives from the id of the split instance, never from the round number: the
// number depends on commit order, the id does not.
func roundTokenID(exec ID, join flow.NodeID, scope ScopePath, split NodeExecutionID) TokenID {
	return TokenID(hashID(string(exec), string(join), string(scope), "round", string(split)))
}

// nodeExecutionID is H(executionId, tokenId, nodeId, scopePath).
func nodeExecutionID(exec ID, token TokenID, node flow.NodeID, scope ScopePath) NodeExecutionID {
	return NodeExecutionID(hashID(string(exec), string(token), string(node), string(scope)))
}

// outputRef names the batch a node execution emitted on a port (ADR-0008
// section 3: one document per node execution and port).
func outputRef(ne NodeExecutionID, port string) PayloadRef {
	return PayloadRef(hashID("payload", string(ne), "out", port))
}

// inputRef names the input batch of an execution.
func inputRef(exec ID) PayloadRef {
	return PayloadRef(hashID("payload", string(exec), "input"))
}

// joinedRef names the combined input batch of a join continuation token.
func joinedRef(token TokenID) PayloadRef {
	return PayloadRef(hashID("payload", string(token), "joined"))
}
