// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/egress"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/connector/declarative"
	httpdispatch "github.com/wirvii/orcon/internal/connector/dispatch"
	"github.com/wirvii/orcon/internal/credential"
	"github.com/wirvii/orcon/internal/credential/envelope"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/trigger"
	"github.com/wirvii/orcon/internal/trigger/triggertest"
	"github.com/wirvii/orcon/test/mockprovider"
)

// TestWebhookRunsIfThenHTTPToSucceeded is the first end-to-end run of the engine
// (SPEC-002 T2 with SPEC-001 C4 and SPEC-003 CN3): a published webhook flow
// starts an execution through the HTTP router, an IF node routes the event by
// its amount, and the true branch calls the mock provider through orcon.http@1.
// Both stores run the same test.
func TestWebhookRunsIfThenHTTPToSucceeded(t *testing.T) {
	for _, kind := range e2eKinds() {
		t.Run(string(kind), func(t *testing.T) {
			runWebhookFlow(t, kind, false)
		})
	}
}

// TestWebhookCallsWithAStoredCredential runs the same flow with the credential in the credential store: the
// HTTP node resolves it through the credential service for the project of the execution, and the request
// carries its secret.
func TestWebhookCallsWithAStoredCredential(t *testing.T) {
	for _, kind := range e2eKinds() {
		t.Run(string(kind), func(t *testing.T) {
			runWebhookFlow(t, kind, true)
		})
	}
}

// e2eKinds lists the stores the end-to-end test runs on: SQLite always, and
// PostgreSQL when the test server is on (ORCON_TEST_POSTGRES).
func e2eKinds() []storetest.Kind {
	if storetest.PostgresEnabled() {
		return []storetest.Kind{storetest.SQLite, storetest.PostgreSQL}
	}
	return []storetest.Kind{storetest.SQLite}
}

func runWebhookFlow(t *testing.T, kind storetest.Kind, stored bool) {
	t.Helper()
	ctx := t.Context()
	env := storetest.NewEngineEnv(t, kind, storetest.EngineOptions{})
	st, ok := env.Store.(engineStore)
	if !ok {
		t.Fatalf("the %s store does not implement the engine store", kind)
	}

	provider := mockprovider.New()
	srv := httptest.NewServer(provider)
	t.Cleanup(srv.Close)

	opts := engineOptions{
		role: config.RoleStandalone, logger: slog.New(slog.DiscardHandler), version: "test",
		senderPolicy: func(u *url.URL) egress.Policy {
			return egress.Policy{Domains: []string{u.Hostname()}, Schemes: []string{"http"}, AllowPrivateNetworks: true}
		},
		credentials: testCredentials,
	}
	credentialRef := "mock"
	if stored {
		svc, ref := storedCredential(t, env, srv.URL)
		opts.credentials, opts.resolver, credentialRef = nil, svc, ref
	}
	eng, err := newEngine(st, opts)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	runCtx, stopEngine := context.WithCancel(ctx)
	engineDone := make(chan error, 1)
	go func() { engineDone <- eng.run(runCtx) }()
	t.Cleanup(func() {
		stopEngine()
		if err := <-engineDone; err != nil {
			t.Errorf("engine stopped with error: %v", err)
		}
	})

	publishWebhookFlow(t, env, eng, srv.URL, credentialRef)

	handler, err := newAPIRouterWith(config.RoleStandalone, slog.New(slog.DiscardHandler), nil, eng.ingress())
	if err != nil {
		t.Fatalf("build the router: %v", err)
	}

	// The true branch: the amount is above the threshold, the provider is called.
	exec := postEvent(t, handler, "/hooks/engine/orders", `{"amount": 42}`, http.StatusAccepted)
	waitExecution(t, env, st, exec, execution.ExecSucceeded)
	checkExecutionReads(t, env, st, exec)
	if n := len(provider.Requests()); n != 1 {
		t.Fatalf("the provider received %d requests, want 1", n)
	}
	if got := provider.Requests()[0].Authorization; got != "Bearer "+mockprovider.Token {
		t.Fatalf("the provider received Authorization %q", got)
	}

	// The false branch: the amount is below the threshold, nothing is called.
	exec = postEvent(t, handler, "/hooks/engine/orders", `{"amount": 1}`, http.StatusAccepted)
	waitExecution(t, env, st, exec, execution.ExecSucceeded)
	if n := len(provider.Requests()); n != 1 {
		t.Fatalf("the false branch called the provider: %d requests", n)
	}

	postEvent(t, handler, "/hooks/engine/unknown", `{"amount": 1}`, http.StatusNotFound)
	postEvent(t, handler, "/hooks/engine/orders", `{"amount": `, http.StatusBadRequest)
}

// testCredentials resolves the one credential the test flow names: a bearer
// token the mock provider accepts.
func testCredentials(_ context.Context, req httpdispatch.CredentialRequest) (httpdispatch.Credential, error) {
	if req.ID != "mock" {
		return httpdispatch.Credential{}, errors.New("unknown test credential")
	}
	return httpdispatch.Credential{
		Auth:   declarative.Auth{Type: declarative.AuthBearer, Field: "token"},
		Fields: map[string]string{"token": mockprovider.Token},
	}, nil
}

// publishWebhookFlow stores a draft, publishes it and activates it, through the
// trigger service the process runs.
func publishWebhookFlow(t *testing.T, env *storetest.EngineEnv, eng *engine, providerURL, credentialRef string) {
	t.Helper()
	ctx := t.Context()
	extra := []string{
		`{"id":"check","name":"Check","element":"exclusiveGateway","type":"orcon.if@1","routing":"EXCLUSIVE","sensitivePaths":["/amount"],"parameters":{"condition":"={{ $json.amount > 10 }}"}}`,
		fmt.Sprintf(`{"id":"call","name":"Call","element":"serviceTask","type":"orcon.http@1","parameters":{"method":"GET","url":%q,"credential":%q}}`,
			providerURL+"/v1/items?offset=0&limit=2", credentialRef),
	}
	extraEdges := []string{
		`{"id":"e-true","from":{"node":"check","port":"true"},"to":{"node":"call"},"kind":"SEQUENCE"}`,
		`{"id":"e-false","from":{"node":"check","port":"false"},"to":{"node":"end"},"kind":"SEQUENCE"}`,
		`{"id":"e-call","from":{"node":"call","port":"main"},"to":{"node":"end"},"kind":"SEQUENCE"}`,
	}
	draft := triggertest.Build(triggertest.DocSpec{
		Name: "webhook-if-http",
		Starts: []triggertest.Node{{
			ID: "hook", Type: triggertest.TypeWebhook, Block: triggertest.WebhookBlock("orders"), Next: "check",
		}},
		Extra:      extra,
		ExtraEdges: extraEdges,
	})
	now := env.Clock.Now()
	rec := flow.Record{
		ID: flow.FlowID(env.IDs.New()), OrgID: env.Org, ProjectID: env.Project, Name: "webhook-if-http",
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

// postEvent sends one public request and checks its status; it returns the
// execution the request started, for a 202.
func postEvent(t *testing.T, h http.Handler, path, body string, want int) store.ExecutionID {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("POST %s: status %d, want %d: %s", path, rec.Code, want, rec.Body.String())
	}
	if want != http.StatusAccepted {
		return store.ExecutionID{}
	}
	u, err := id.Parse(rec.Header().Get("Orcon-Execution-Id"))
	if err != nil {
		t.Fatalf("the response names no execution: %v", err)
	}
	return store.ExecutionID(u)
}

// waitExecution polls the store until the execution reaches the state.
func waitExecution(t *testing.T, env *storetest.EngineEnv, st engineStore, exec store.ExecutionID, want execution.State) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		ex, err := st.GetExecution(t.Context(), storetest.ScopeOf(env.Org), exec)
		if err == nil {
			last = ex.Status
			if last == string(want) {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	events := env.Events(exec)
	for i := range events {
		t.Logf("event %s", events[i].Type)
	}
	t.Fatalf("execution %s stayed in %q, want %s", exec, last, want)
}

// storedCredential keeps the mock provider's bearer token in the credential store of env, bound to the host of
// the provider, and returns the service and the identifier the flow names.
func storedCredential(t *testing.T, env *storetest.EngineEnv, providerURL string) (*credential.Service, string) {
	t.Helper()
	ctx := t.Context()
	u, err := url.Parse(providerURL)
	if err != nil {
		t.Fatal(err)
	}
	keys, ok := env.Store.(interface {
		credential.Store
		envelope.KeyStore
		CreatePrincipal(context.Context, identity.PrincipalRecord) error
	})
	if !ok {
		t.Fatalf("the %s store does not implement the credential store", env.Kind)
	}
	kr := envelope.NewKeyring(envelope.NewLocalMasterKey("file", "e2e-master", mustMasterKey(t)), keys)
	if err := kr.InitMasterCanary(ctx); err != nil {
		t.Fatalf("master canary: %v", err)
	}
	if _, err := kr.CreateKey(ctx, envelope.OrgScope(env.Org)); err != nil {
		t.Fatalf("organization key: %v", err)
	}
	svc, err := credential.New(credential.Config{Store: keys, Keys: kr, Clock: env.Clock, IDs: env.IDs})
	if err != nil {
		t.Fatalf("credential service: %v", err)
	}
	now := env.Clock.Now()
	owner := identity.PrincipalID(env.IDs.New())
	if err := keys.CreatePrincipal(ctx, identity.PrincipalRecord{ID: owner, Kind: identity.PrincipalUser, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("credential owner: %v", err)
	}
	created, err := svc.Create(ctx, storetest.ScopeOf(env.Org), env.Project, credential.Input{
		Name: "Mock provider", ConnectorReference: "http", AllowedHosts: []string{u.Hostname()},
		Secret: map[string]string{"token": mockprovider.Token},
	}, audit.Actor{PrincipalID: id.UUID(owner), Kind: audit.ActorUser})
	if err != nil {
		t.Fatalf("create the credential: %v", err)
	}
	return svc, created.ID.String()
}

// mustMasterKey returns a fresh master key for the test keyring.
func mustMasterKey(t *testing.T) envelope.Key {
	t.Helper()
	key, err := envelope.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("master key: %v", err)
	}
	return key
}

// TestResolveCredentialsBindsTheHost: the credential resolves for the host it is bound to and for no other.
func TestResolveCredentialsBindsTheHost(t *testing.T) {
	for _, kind := range e2eKinds() {
		t.Run(string(kind), func(t *testing.T) {
			env := storetest.NewEngineEnv(t, kind, storetest.EngineOptions{})
			svc, ref := storedCredential(t, env, "http://provider.example.com")
			st, ok := env.Store.(engineStore)
			if !ok {
				t.Fatalf("the %s store does not implement the engine store", kind)
			}
			resolve := resolveCredentials(svc, st)
			req := httpdispatch.CredentialRequest{Org: env.Org, Project: env.Project, ID: ref, Host: "provider.example.com"}
			got, err := resolve(t.Context(), req)
			if err != nil || got.Auth.Type != declarative.AuthBearer || got.Fields["token"] != mockprovider.Token {
				t.Fatalf("bound host: %+v, %v; want a bearer credential", got.Auth, err)
			}
			req.Host = "evil.example.net"
			if _, err := resolve(t.Context(), req); !errors.Is(err, credential.ErrHostNotAllowed) {
				t.Fatalf("other host: %v; want ErrHostNotAllowed", err)
			}
			req.Host, req.Project = "provider.example.com", identity.ProjectID(env.IDs.New())
			if _, err := resolve(t.Context(), req); !errors.Is(err, credential.ErrNotUsable) {
				t.Fatalf("project without a share: %v; want ErrNotUsable", err)
			}
			req.Project, req.ID = env.Project, "not-an-id"
			if _, err := resolve(t.Context(), req); !errors.Is(err, credential.ErrNotFound) {
				t.Fatalf("malformed id: %v; want ErrNotFound", err)
			}
		})
	}
}
