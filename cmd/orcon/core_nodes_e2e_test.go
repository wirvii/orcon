// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi"
	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/trigger"
	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

// TestCoreNodesRunFromAWebhookOnEveryStore is the end-to-end run of the core
// data nodes through the wiring of the binary: a webhook flow with Filter, Sort
// and Limit receives a list, and the data stored for the last node holds the
// filtered, sorted and limited items. Both stores run the same test.
func TestCoreNodesRunFromAWebhookOnEveryStore(t *testing.T) {
	for _, kind := range e2eKinds() {
		t.Run(string(kind), func(t *testing.T) {
			env, st, eng, handler := coreEngine(t, kind)
			publishCoreFlow(t, env, eng, "core-ok",
				`{"id":"keep","name":"Keep","element":"task","type":"orcon.filter@1","parameters":{"condition":"$json.age >= 18"}}`,
				`{"id":"order","name":"Order","element":"task","type":"orcon.sort@1","parameters":{"fields":[{"field":"age","order":"DESCENDING"}]}}`,
				`{"id":"top","name":"Top","element":"task","type":"orcon.limit@1","parameters":{"maxItems":2}}`,
			)

			exec := postEvent(t, handler, "/hooks/engine/core-ok",
				`[{"name":"a","age":17},{"name":"b","age":30},{"name":"c","age":18},{"name":"d","age":45},{"name":"e","age":5}]`,
				http.StatusAccepted)
			waitExecution(t, env, st, exec, execution.ExecSucceeded)
			got := nodeData(t, env, st, exec, "top")
			want := `[{"age":45,"name":"d"},{"age":30,"name":"b"}]`
			if got != want {
				t.Errorf("the data of the last node = %s, want %s", got, want)
			}
		})
	}
}

// TestCoreNodeFailureEndsTheExecutionOnEveryStore: a condition that is not a
// boolean fails the Filter node for good, and the execution ends FAILED on each
// store.
func TestCoreNodeFailureEndsTheExecutionOnEveryStore(t *testing.T) {
	for _, kind := range e2eKinds() {
		t.Run(string(kind), func(t *testing.T) {
			env, st, eng, handler := coreEngine(t, kind)
			publishCoreFlow(t, env, eng, "core-bad",
				`{"id":"keep","name":"Keep","element":"task","type":"orcon.filter@1","parameters":{"condition":"$json.age"}}`,
			)
			exec := postEvent(t, handler, "/hooks/engine/core-bad", `[{"age":20}]`, http.StatusAccepted)
			waitExecution(t, env, st, exec, execution.ExecFailed)
		})
	}
}

// coreEngine starts the engine of the binary over a store of the kind.
func coreEngine(t *testing.T, kind storetest.Kind) (*storetest.EngineEnv, engineStore, *engine, http.Handler) { //nolint:ireturn // the engine store is the interface the engine is built over
	t.Helper()
	env := storetest.NewEngineEnv(t, kind, storetest.EngineOptions{})
	st, ok := env.Store.(engineStore)
	if !ok {
		t.Fatalf("the %s store does not implement the engine store", kind)
	}
	eng, err := newEngine(st, engineOptions{
		role: config.RoleStandalone, logger: slog.New(slog.DiscardHandler), version: "test",
		credentials: testCredentials,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	runCtx, stopEngine := context.WithCancel(t.Context())
	engineDone := make(chan error, 1)
	go func() { engineDone <- eng.run(runCtx) }()
	t.Cleanup(func() {
		stopEngine()
		if err := <-engineDone; err != nil {
			t.Errorf("engine stopped with error: %v", err)
		}
	})
	handler, err := newAPIRouterWith(config.RoleStandalone, slog.New(slog.DiscardHandler), nil, eng.ingress())
	if err != nil {
		t.Fatalf("build the router: %v", err)
	}
	return env, st, eng, handler
}

// publishCoreFlow stores a draft whose webhook (named like the flow) leads to
// the given nodes, one after the other and then to the end event, publishes it
// and activates it. The nodes are JSON texts with an id.
func publishCoreFlow(t *testing.T, env *storetest.EngineEnv, eng *engine, name string, nodes ...string) {
	t.Helper()
	ctx := t.Context()
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		var probe struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(n), &probe); err != nil {
			t.Fatalf("node %s: %v", n, err)
		}
		ids = append(ids, probe.ID)
	}
	var edges []string
	for i := 1; i < len(ids); i++ {
		edges = append(edges, `{"id":"e-`+ids[i-1]+`","from":{"node":"`+ids[i-1]+`","port":"main"},"to":{"node":"`+ids[i]+`"},"kind":"SEQUENCE"}`)
	}
	last := ids[len(ids)-1]
	edges = append(edges, `{"id":"e-`+last+`","from":{"node":"`+last+`","port":"main"},"to":{"node":"end"},"kind":"SEQUENCE"}`)
	draft := triggertest.Build(triggertest.DocSpec{
		Name: name,
		Starts: []triggertest.Node{{
			ID: "hook", Type: triggertest.TypeWebhook, Block: triggertest.WebhookBlock(name), Next: ids[0],
		}},
		Extra:      nodes,
		ExtraEdges: edges,
	})
	now := env.Clock.Now()
	rec := flow.Record{
		ID: flow.FlowID(env.IDs.New()), OrgID: env.Org, ProjectID: env.Project, Name: name,
		Draft: draft, CreatedAt: now, UpdatedAt: now,
	}
	if err := env.Store.CreateFlow(ctx, storetest.ScopeOf(env.Org), rec); err != nil {
		t.Fatalf("create flow: %v", err)
	}
	by := identity.PrincipalID(env.IDs.New())
	if err := env.Store.CreatePrincipal(ctx, identity.PrincipalRecord{ID: by, Kind: identity.PrincipalUser, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("create the publishing principal: %v", err)
	}
	if _, err := eng.trigger.Publish(ctx, trigger.PublishCommand{Org: storetest.ScopeOf(env.Org), Flow: rec.ID, By: by}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := eng.trigger.Activate(ctx, storetest.ScopeOf(env.Org), rec.ID); err != nil {
		t.Fatalf("activate: %v", err)
	}
}

// nodeData reads, through the execution reads of the API, the items a node
// emitted, as compact JSON with the members of each item sorted by name.
func nodeData(t *testing.T, env *storetest.EngineEnv, st engineStore, exec store.ExecutionID, node string) string {
	t.Helper()
	reader, ok := env.Store.(httpapi.ExecutionReader)
	if !ok {
		t.Fatalf("the %s store does not implement the execution reads", env.Kind)
	}
	packages, ok := env.Store.(httpapi.ConnectorPackages)
	if !ok {
		t.Fatalf("the %s store does not implement the connector package reads", env.Kind)
	}
	codec, err := page.NewCodec(make([]byte, page.MinKeyLength))
	if err != nil {
		t.Fatal(err)
	}
	h := httpapi.NewExecutions(reader, st, flowReader{}, packages, allowAll{}, &recordingAudit{}, env.Clock, env.IDs, codec)
	principal := identity.Principal{ID: identity.PrincipalID(env.IDs.New()), Kind: identity.PrincipalUser, UserID: identity.UserID(env.IDs.New()), CredentialType: identity.CredentialSession}
	ctx := authz.WithOrgScope(middleware.WithPrincipal(t.Context(), principal), storetest.ScopeOf(env.Org))
	data, err := h.ListExecutionData(ctx, apigen.ListExecutionDataRequestObject{ExecutionId: apigen.ExecutionId(exec)})
	if err != nil {
		t.Fatalf("list data: %v", err)
	}
	for _, entry := range answer[apigen.ListExecutionData200JSONResponse](t, data).Body.Items {
		if entry.NodeId != node {
			continue
		}
		raw, err := json.Marshal(entry.DataItems)
		if err != nil {
			t.Fatal(err)
		}
		return compactSorted(t, raw)
	}
	t.Fatalf("no data for the node %q", node)
	return ""
}

// compactSorted re-encodes JSON text with the members of every object sorted.
func compactSorted(t *testing.T, raw []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}
