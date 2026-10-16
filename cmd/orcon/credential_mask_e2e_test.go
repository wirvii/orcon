// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/egress"
	"github.com/wirvii/orcon/internal/adapter/httpapi"
	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/config"
)

// echoProvider answers every request with what it received: the credential headers, as a host that
// echoes its request does (httpbin-style endpoints, error messages that quote the key). It also puts
// the Authorization header into a key of the response, so the masking of keys is exercised.
func echoProvider() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		body, _ := json.Marshal(map[string]any{
			"echo":         map[string]string{"authorization": auth, "apiKey": r.Header.Get("X-API-Key")},
			"seen " + auth: true,
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
}

// TestResolvedSecretsNeverReachStoredRunData is the echo test of ADR-0043 section 5 and R-CRED-04: a host
// that echoes the credential back must not put the secret in the node output, in the journal response
// or in what GET /executions/{id}/data serves. It runs on every store the end-to-end tests run on, for a
// read (GET) and an effect (POST, journaled), and for the three schemes the HTTP node applies.
func TestResolvedSecretsNeverReachStoredRunData(t *testing.T) {
	const bearer, apiKey, user, pass = "SUPERSECRET-bearer-123", "SUPERSECRET-apikey-456", "SUPERSECRET-user", "SUPERSECRET-pass"
	cases := []struct {
		name      string
		secret    map[string]string
		forbidden []string
	}{
		{"bearer", map[string]string{"token": bearer}, []string{bearer}},
		{"apikey", map[string]string{"apiKey": apiKey}, []string{apiKey}},
		{"basic", map[string]string{"username": user, "password": pass},
			[]string{user, pass, base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))}},
	}
	for _, kind := range e2eKinds() {
		for _, tc := range cases {
			for _, method := range []string{"GET", "POST"} {
				t.Run(string(kind)+"/"+tc.name+"/"+method, func(t *testing.T) {
					runEchoFlow(t, kind, tc.secret, method, tc.forbidden)
				})
			}
		}
	}
}

func runEchoFlow(t *testing.T, kind storetest.Kind, secret map[string]string, method string, forbidden []string) {
	t.Helper()
	ctx := t.Context()
	env := storetest.NewEngineEnv(t, kind, storetest.EngineOptions{})
	st, ok := env.Store.(engineStore)
	if !ok {
		t.Fatalf("the %s store does not implement the engine store", kind)
	}
	srv := httptest.NewServer(echoProvider())
	t.Cleanup(srv.Close)

	svc, ref := storedCredentialWith(t, env, srv.URL, secret)
	eng, err := newEngine(st, engineOptions{
		role: config.RoleStandalone, logger: slog.New(slog.DiscardHandler), version: "test",
		senderPolicy: func(u *url.URL) egress.Policy {
			return egress.Policy{Domains: []string{u.Hostname()}, Schemes: []string{"http"}, AllowPrivateNetworks: true}
		},
		resolver: svc,
	})
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
	publishWebhookFlow(t, env, eng, srv.URL+"/echo", ref, method)
	handler, err := newAPIRouterWith(config.RoleStandalone, slog.New(slog.DiscardHandler), nil, eng.ingress())
	if err != nil {
		t.Fatalf("build the router: %v", err)
	}
	exec := postEvent(t, handler, "/hooks/engine/orders", `{"amount": 42}`, http.StatusAccepted)
	waitExecution(t, env, st, exec, execution.ExecSucceeded)

	scope := storetest.ScopeOf(env.Org)
	var stored strings.Builder

	// What GET /executions/{id}/data serves.
	codec, err := page.NewCodec(make([]byte, page.MinKeyLength))
	if err != nil {
		t.Fatal(err)
	}
	reader, ok := env.Store.(httpapi.ExecutionReader)
	if !ok {
		t.Fatalf("the %s store does not implement the execution reads", kind)
	}
	packages, ok := env.Store.(httpapi.ConnectorPackages)
	if !ok {
		t.Fatalf("the %s store does not implement the connector package reads", kind)
	}
	h := httpapi.NewExecutions(reader, st, flowReader{}, packages, allowAll{}, &recordingAudit{}, env.Clock, env.IDs, codec)
	principal := identity.Principal{ID: identity.PrincipalID(env.IDs.New()), Kind: identity.PrincipalUser, UserID: identity.UserID(env.IDs.New()), CredentialType: identity.CredentialSession}
	actx := authz.WithOrgScope(middleware.WithPrincipal(ctx, principal), scope)
	data, err := h.ListExecutionData(actx, apigen.ListExecutionDataRequestObject{ExecutionId: apigen.ExecutionId(exec)})
	if err != nil {
		t.Fatalf("list data: %v", err)
	}
	raw, err := json.Marshal(answer[apigen.ListExecutionData200JSONResponse](t, data).Body)
	if err != nil {
		t.Fatal(err)
	}
	dataText := string(raw)
	stored.WriteString(dataText)
	if !strings.Contains(dataText, "echo") || !strings.Contains(dataText, "[REDACTED]") {
		t.Fatalf("the call node output is not in the served data, or nothing was redacted: %s", dataText)
	}

	// What the journal kept as the response of each effect.
	nodes, err := reader.ListNodeExecutions(ctx, scope, exec, store.Cursor{}, 100)
	if err != nil {
		t.Fatalf("list node executions: %v", err)
	}
	journaled := 0
	for i := range nodes {
		ops, err := st.ListExternalOperations(ctx, scope, nodes[i].ID, 0, store.ExternalOperationID{}, 100)
		if err != nil {
			t.Fatalf("list external operations: %v", err)
		}
		for j := range ops {
			journaled++
			stored.WriteString(ops[j].ResponseReference)
		}
	}
	if wantJournal := method != "GET"; wantJournal != (journaled > 0) {
		t.Fatalf("%s call kept %d journal entries", method, journaled)
	}

	for _, s := range forbidden {
		if strings.Contains(stored.String(), s) {
			t.Fatalf("the stored run data holds the secret %q: %s", s, stored.String())
		}
	}
}
