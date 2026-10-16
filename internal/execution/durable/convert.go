// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// The interpreter names its rows with the text of the deterministic ids
// (ADR-0008 section 6); the store keeps them as UUIDs. These conversions are the
// one place the two meet.

func parseUUID(text string) (id.UUID, error) {
	u, err := id.Parse(text)
	if err != nil {
		return id.Nil, fmt.Errorf("identity %q: %w", text, err)
	}
	return u, nil
}

func tokenID(t execution.TokenID) (store.TokenID, error) {
	u, err := parseUUID(string(t))
	return store.TokenID(u), err
}

func nodeExecutionID(n execution.NodeExecutionID) (store.NodeExecutionID, error) {
	u, err := parseUUID(string(n))
	return store.NodeExecutionID(u), err
}

func payloadID(ref execution.PayloadRef) (store.PayloadID, error) {
	u, err := parseUUID(string(ref))
	return store.PayloadID(u), err
}

func executionText(e store.ExecutionID) execution.ID { return execution.ID(e.String()) }

// rowOf converts a stored token to the interpreter's row.
func rowOf(t *store.Token) (execution.TokenRow, error) {
	row := execution.TokenRow{
		ID: execution.TokenID(t.ID.String()), State: execution.TokenState(t.Status), Node: flow.NodeID(t.NodeID),
		InEdge: flow.EdgeID(t.EdgeID), Scope: execution.ScopePath(t.ScopePath), Payload: execution.PayloadRef(t.PayloadReference),
	}
	if len(t.ForkLineage) > 0 {
		if err := execution.DecodeTokenDocument(t.ForkLineage, &row); err != nil {
			return execution.TokenRow{}, fmt.Errorf("token %s: %w", t.ID, err)
		}
	}
	return row, nil
}

// snapshotOf converts what the store read to what the interpreter opens.
func snapshotOf(exec store.ExecutionID, snap *store.EngineSnapshot) (execution.Snapshot, error) {
	out := execution.Snapshot{
		Execution: executionText(exec), Status: execution.State(snap.Status), Trackers: snap.Document,
		Tokens: make([]execution.TokenRow, 0, len(snap.Tokens)),
	}
	for i := range snap.Tokens {
		row, err := rowOf(&snap.Tokens[i])
		if err != nil {
			return execution.Snapshot{}, err
		}
		out.Tokens = append(out.Tokens, row)
	}
	return out, nil
}

// envDocument writes the environment of an activation (orcon/value@1).
func envDocument(env execution.NodeEnv) ([]byte, error) {
	obj, err := jsonvalue.Object(
		jsonvalue.Member{Key: "time", Value: jsonvalue.Int(env.Time.UnixMicro())},
		jsonvalue.Member{Key: "seed", Value: jsonvalue.String(strconv.FormatUint(env.Seed, 16))},
	)
	if err != nil {
		return nil, fmt.Errorf("encode activation environment: %w", err)
	}
	out, err := jsonvalue.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("encode activation environment: %w", err)
	}
	return out, nil
}

func parseEnv(doc []byte) (execution.NodeEnv, error) {
	v, err := jsonvalue.Parse(doc)
	if err != nil {
		return execution.NodeEnv{}, fmt.Errorf("decode activation environment: %w", err)
	}
	var env execution.NodeEnv
	if t, ok := v.Get("time"); ok {
		if micros, ok := t.Int64(); ok && micros != 0 {
			env.Time = time.UnixMicro(micros).UTC()
		}
	}
	if s, ok := v.Get("seed"); ok {
		text, _ := s.Str()
		seed, err := strconv.ParseUint(text, 16, 64)
		if err != nil {
			return execution.NodeEnv{}, fmt.Errorf("decode activation environment: seed %q: %w", text, err)
		}
		env.Seed = seed
	}
	return env, nil
}

// orgKey carries the organization of the execution being worked, for the
// payload reader: the interpreter's [execution.Payloads] port has no
// organization parameter (go-style.md GS-C6: request-scoped identity in the
// context under an unexported key).
type orgKey struct{}

func withOrg(ctx context.Context, org identity.OrgID) context.Context {
	return context.WithValue(ctx, orgKey{}, org)
}

func orgFrom(ctx context.Context) (identity.OrgID, bool) {
	org, ok := ctx.Value(orgKey{}).(identity.OrgID)
	return org, ok
}

// txKey carries the transaction a recomputation runs in, so that its payload
// reads see the commits that share the transaction.
type txKey struct{}

func withTx(ctx context.Context, tx store.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// payloadReader implements the read half of [execution.Payloads] over the store.
type payloadReader struct {
	reader store.EngineReader
}

// Get implements [execution.Payloads].
func (p payloadReader) Get(ctx context.Context, ref execution.PayloadRef) (execution.Batch, error) {
	org, ok := orgFrom(ctx)
	if !ok {
		return execution.Batch{}, fmt.Errorf("get payload %s: no organization in the context", ref)
	}
	pid, err := payloadID(ref)
	if err != nil {
		return execution.Batch{}, err
	}
	var doc []byte
	if tx, ok := ctx.Value(txKey{}).(store.Tx); ok {
		doc, err = tx.GetPayload(ctx, org, pid)
	} else {
		doc, err = p.reader.GetPayload(ctx, org, pid)
	}
	if errors.Is(err, flow.ErrNotFound) {
		return execution.Batch{}, fmt.Errorf("get payload %s: %w: %w", ref, execution.ErrPayloadNotFound, err)
	}
	if err != nil {
		return execution.Batch{}, fmt.Errorf("get payload %s: %w", ref, err)
	}
	return execution.DecodeBatch(doc)
}

// Put implements [execution.Payloads]. A session writes through its deltas, so
// the interpreter never calls it on the base store.
func (p payloadReader) Put(_ context.Context, ref execution.PayloadRef, _ execution.Batch) error {
	return fmt.Errorf("put payload %s: the durable store takes batches through the commit of a step", ref)
}
