// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"context"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/journal"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// These tests cover the paths an activation takes when the durable engine supplies a
// checkpoint, and when a settled item is read back from the journal, which the
// effect tests reach only through the engine.

func TestStoredResponseReadsWhatTheJournalKept(t *testing.T) {
	if v, err := storedResponse(""); err != nil || v.Kind() != jsonvalue.KindNull {
		t.Fatalf("an empty response = %v, %v", v.Kind(), err)
	}
	v, err := storedResponse(`{"id":7}`)
	if err != nil || v.Kind() != jsonvalue.KindObject {
		t.Fatalf("a stored object = %v, %v", v.Kind(), err)
	}
	if _, err := storedResponse("{broken"); err == nil {
		t.Fatal("a broken stored response was read")
	}
}

func TestOutputsOfReusesOrRefusesSettledItems(t *testing.T) {
	rows := []store.ExternalOperation{
		{ItemIndex: 0, State: journal.StateSucceeded, ResponseReference: `{"id":9}`},
	}
	out, err := outputsOf(rows, []*jsonvalue.Value{nil})
	if err != nil || len(out) != 1 {
		t.Fatalf("reused success = %d, %v", len(out), err)
	}
	if out[0].Kind() != jsonvalue.KindObject {
		t.Fatalf("reused value = %v", out[0].Kind())
	}

	fresh := jsonvalue.String("fresh")
	out, err = outputsOf(rows[:1], []*jsonvalue.Value{&fresh})
	if err != nil || out[0].Kind() != jsonvalue.KindString {
		t.Fatalf("a send of this run is used directly: %v, %v", out, err)
	}

	uncertain := []store.ExternalOperation{{ItemIndex: 0, State: journal.StateUncertain}}
	_, err = outputsOf(uncertain, []*jsonvalue.Value{nil})
	var suspend *execution.SuspendError
	if !errors.Is(err, ErrUnknownOutcome) || !errors.As(err, &suspend) {
		t.Fatalf("an uncertain item = %v, want a suspension that wraps ErrUnknownOutcome", err)
	}
	failed := []store.ExternalOperation{{ItemIndex: 0, State: journal.StateFailed}}
	if _, err := outputsOf(failed, []*jsonvalue.Value{nil}); err == nil {
		t.Fatal("a failed item produced output")
	}
	pending := []store.ExternalOperation{{ItemIndex: 0, State: journal.StatePrepared}}
	if _, err := outputsOf(pending, []*jsonvalue.Value{nil}); err == nil {
		t.Fatal("a pending item produced output")
	}
}

func TestJournalStoreRunsInsideTheCheckpoint(t *testing.T) {
	j := newFakeJournal()
	direct := journalStore{base: j}
	if err := direct.InTx(t.Context(), testScopeOf(t), func(context.Context, store.Tx) error { return nil }); err != nil {
		t.Fatalf("direct transaction: %v", err)
	}

	var inside bool
	checked := journalStore{base: j, checkpoint: func(ctx context.Context, fn func(context.Context, any) error) error {
		return fn(ctx, &fakeJournalTx{j: j})
	}}
	err := checked.InTx(t.Context(), testScopeOf(t), func(context.Context, store.Tx) error {
		inside = true
		return nil
	})
	if err != nil || !inside {
		t.Fatalf("checkpointed transaction: ran=%v err=%v", inside, err)
	}

	notStore := journalStore{base: j, checkpoint: func(ctx context.Context, fn func(context.Context, any) error) error {
		return fn(ctx, 42)
	}}
	if err := notStore.InTx(t.Context(), testScopeOf(t), func(context.Context, store.Tx) error { return nil }); err == nil {
		t.Fatal("a checkpoint with a foreign transaction was accepted")
	}

	if _, err := checked.ListExternalOperations(t.Context(), testScopeOf(t), store.NodeExecutionID(id.Derive("node-execution")), 0, store.ExternalOperationID{}, 10); err != nil {
		t.Fatalf("list through the checkpoint store: %v", err)
	}
}

func TestRouterAndProviderEdges(t *testing.T) {
	r := NewRouter(map[string]execution.NodeRunner{})
	if _, err := r.Execute(t.Context(), execution.NodeCall{Node: httpNode(t, `{}`)}); err == nil {
		t.Fatal("a type without a runner was executed")
	}
	p := &effectProvider{}
	if _, err := p.Lookup(t.Context(), &journal.Attempt{}); err == nil {
		t.Fatal("orcon.http@1 offered a reconciliation")
	}
}

func TestCredentialResolution(t *testing.T) {
	n := &HTTPNode{}
	c, err := n.credential(t.Context(), testOrg, "")
	if err != nil || c.Auth.Type != declarative.AuthNone {
		t.Fatalf("no credential named = %+v, %v", c, err)
	}
	if _, err := n.credential(t.Context(), testOrg, "crm"); err == nil {
		t.Fatal("a named credential without a resolver was accepted")
	}
	n.cfg.Credential = func(context.Context, identity.OrgID, string) (Credential, error) {
		return Credential{Fields: map[string]string{"token": "t"}}, nil
	}
	c, err = n.credential(t.Context(), testOrg, "crm")
	if err != nil || c.Fields["token"] != "t" {
		t.Fatalf("resolved credential = %+v, %v", c, err)
	}
}

// testScopeOf is the scope of the test organization.
func testScopeOf(t *testing.T) orgscope.Org {
	t.Helper()
	s, err := testScopes{}.ScopeOf(testOrg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
