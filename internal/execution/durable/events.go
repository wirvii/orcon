// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// docBuilder writes one JSON object, leaving out the members that hold no
// value.
type docBuilder struct{ members []jsonvalue.Member }

func (b *docBuilder) str(key, s string) {
	if s != "" {
		b.members = append(b.members, jsonvalue.Member{Key: key, Value: jsonvalue.String(s)})
	}
}

func (b *docBuilder) num(key string, n int64) {
	if n != 0 {
		b.members = append(b.members, jsonvalue.Member{Key: key, Value: jsonvalue.Int(n)})
	}
}

func (b *docBuilder) flag(key string, v bool) {
	b.members = append(b.members, jsonvalue.Member{Key: key, Value: jsonvalue.Bool(v)})
}

func (b *docBuilder) strs(key string, items []string) {
	if len(items) == 0 {
		return
	}
	vals := make([]jsonvalue.Value, len(items))
	for i, s := range items {
		vals[i] = jsonvalue.String(s)
	}
	b.members = append(b.members, jsonvalue.Member{Key: key, Value: jsonvalue.Array(vals...)})
}

func (b *docBuilder) bytes() ([]byte, error) {
	obj, err := jsonvalue.Object(b.members...)
	if err != nil {
		return nil, fmt.Errorf("event document: %w", err)
	}
	out, err := jsonvalue.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("event document: %w", err)
	}
	return out, nil
}

// eventRow converts an interpreter event to the row of execution_event. The
// reference document carries the envelope identities of ADR-0009 section 7
// (token, node execution, node, scope), the data document the rest.
func eventRow(org identity.OrgID, exec *store.Execution, e *execution.Event, now time.Time, ids id.Generator) (store.Event, error) {
	var ref, data docBuilder
	ref.str("tokenId", string(e.Token))
	ref.str("nodeExecutionId", string(e.NodeExecution))
	ref.str("nodeId", string(e.Node))
	ref.str("scopePath", string(e.Scope))
	ref.str("edgeId", string(e.Edge))
	switch e.Type {
	case execution.EventEdgeClosed:
		data.flag("live", e.Live)
	case execution.EventLoopIterationStarted, execution.EventLoopLimitExceeded:
		data.num("iteration", int64(e.Iteration))
	case execution.EventJoinFired, execution.EventJoinFailed:
		data.num("round", int64(e.Round))
	case execution.EventJoinReset:
		data.num("round", int64(e.Round))
		data.num("liveArrivalCount", int64(e.LiveArrivals))
		data.num("arrivalCount", int64(e.Arrivals))
	case execution.EventExecutionStarted, execution.EventExecutionResumed, execution.EventExecutionSucceeded, execution.EventExecutionFailed,
		execution.EventTokenForked, execution.EventTokenDeferred, execution.EventTokenConsumed,
		execution.EventNodeStarted, execution.EventNodeSucceeded, execution.EventNodeFailed:
	}
	tokens := make([]string, len(e.Tokens))
	for i, t := range e.Tokens {
		tokens[i] = string(t)
	}
	data.strs("tokens", tokens)
	data.str("detail", e.Detail)
	refDoc, err := ref.bytes()
	if err != nil {
		return store.Event{}, err
	}
	var dataDoc []byte
	if len(data.members) > 0 {
		if dataDoc, err = data.bytes(); err != nil {
			return store.Event{}, err
		}
	}
	return store.Event{
		ID: store.EventID(ids.New()), OrgID: org, ExecutionID: exec.ID, RootExecutionID: exec.RootExecutionID,
		OccurredAt: now, Type: string(e.Type), SchemaVersion: 1, Reference: refDoc, Data: dataDoc, CreatedAt: now,
	}, nil
}
