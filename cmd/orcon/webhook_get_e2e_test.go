// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/trigger"
	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

// TestNativeWebhookAcceptingGETStartsAnExecution: GET is an allowed method of a
// native webhook address. The WhatsApp subscription handshake must not swallow
// it: a GET to a route that declares no handshake is an ordinary event that
// starts an execution, on both stores.
func TestNativeWebhookAcceptingGETStartsAnExecution(t *testing.T) {
	for _, kind := range e2eKinds() {
		t.Run(string(kind), func(t *testing.T) {
			env := storetest.NewEngineEnv(t, kind, storetest.EngineOptions{})
			st, ok := env.Store.(engineStore)
			if !ok {
				t.Fatalf("the %s store does not implement the engine store", kind)
			}
			eng, err := newEngine(st, engineOptions{
				role: config.RoleStandalone, logger: slog.New(slog.DiscardHandler), version: "test",
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

			publishGETWebhookFlow(t, env, eng)
			handler, err := newAPIRouterWith(config.RoleStandalone, slog.New(slog.DiscardHandler), nil, eng.ingress())
			if err != nil {
				t.Fatalf("build the router: %v", err)
			}

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hooks/engine/lookup", http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusAccepted {
				t.Fatalf("GET /hooks/engine/lookup: status %d, want 202: %s", rec.Code, rec.Body.String())
			}
			u, err := id.Parse(rec.Header().Get("Orcon-Execution-Id"))
			if err != nil {
				t.Fatalf("the response names no execution: %v", err)
			}
			waitExecution(t, env, st, store.ExecutionID(u), execution.ExecSucceeded)

			// A GET to an address nobody serves is still not found, not forbidden.
			req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hooks/engine/unknown", http.NoBody)
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("GET to an unknown address: status %d, want 404", rec.Code)
			}
		})
	}
}

// publishGETWebhookFlow stores, publishes and activates a flow whose webhook
// accepts GET, through the trigger service the process runs.
func publishGETWebhookFlow(t *testing.T, env *storetest.EngineEnv, eng *engine) {
	t.Helper()
	ctx := t.Context()
	draft := triggertest.Build(triggertest.DocSpec{
		Name: "webhook-get",
		Starts: []triggertest.Node{{
			ID: "hook", Type: triggertest.TypeWebhook,
			Block: `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"lookup","methods":["GET"]}]}`,
		}},
	})
	now := env.Clock.Now()
	rec := flow.Record{
		ID: flow.FlowID(env.IDs.New()), OrgID: env.Org, ProjectID: env.Project, Name: "webhook-get",
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
