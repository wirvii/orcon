// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
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
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/config"
)

const examplePage = `<!doctype html><html><head><title>Example Domain</title></head><body><h1>Example Domain</h1></body></html>`

// TestWebhookCallsAPageThatIsNotJSON is the end-to-end case of BL-146: a webhook starts a flow whose
// orcon.http@1 node reads an HTML page. The execution SUCCEEDED, and the text of the page is in the node
// output, as an n8n HTTP Request does with its default response format. The same run on both stores.
func TestWebhookCallsAPageThatIsNotJSON(t *testing.T) {
	cases := []struct {
		name, path string
		want       []string
	}{
		{"html", "/page", []string{`"data"`, "Example Domain", "<h1>"}},
		{"binary", "/image", []string{`"binary"`, `"mimeType":"image/png"`, "iVBORw0K"}},
		{"json", "/json", []string{`"id":7`}},
	}
	for _, kind := range e2eKinds() {
		for _, tc := range cases {
			t.Run(string(kind)+"/"+tc.name, func(t *testing.T) {
				runFormatFlow(t, kind, tc.path, tc.want)
			})
		}
	}
}

func formatProvider() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page":
			w.Header().Set("Content-Type", "text/html; charset=UTF-8")
			_, _ = w.Write([]byte(examplePage))
		case "/image":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n\x00\x00"))
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":7}`))
		default:
			http.NotFound(w, r)
		}
	})
}

func runFormatFlow(t *testing.T, kind storetest.Kind, path string, want []string) {
	t.Helper()
	ctx := t.Context()
	env := storetest.NewEngineEnv(t, kind, storetest.EngineOptions{})
	st, ok := env.Store.(engineStore)
	if !ok {
		t.Fatalf("the %s store does not implement the engine store", kind)
	}
	srv := httptest.NewServer(formatProvider())
	t.Cleanup(srv.Close)

	eng, err := newEngine(st, engineOptions{
		role: config.RoleStandalone, logger: slog.New(slog.DiscardHandler), version: "test",
		senderPolicy: func(u *url.URL) egress.Policy {
			return egress.Policy{Domains: []string{u.Hostname()}, Schemes: []string{"http"}, AllowPrivateNetworks: true}
		},
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
	publishWebhookFlow(t, env, eng, srv.URL+path, "", "GET")
	handler, err := newAPIRouterWith(config.RoleStandalone, slog.New(slog.DiscardHandler), nil, eng.ingress())
	if err != nil {
		t.Fatalf("build the router: %v", err)
	}
	exec := postEvent(t, handler, "/hooks/engine/orders", `{"amount": 42}`, http.StatusAccepted)
	waitExecution(t, env, st, exec)

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
	actx := authz.WithOrgScope(middleware.WithPrincipal(ctx, principal), storetest.ScopeOf(env.Org))
	data, err := h.ListExecutionData(actx, apigen.ListExecutionDataRequestObject{ExecutionId: apigen.ExecutionId(exec)})
	if err != nil {
		t.Fatalf("list data: %v", err)
	}
	encoded, err := json.Marshal(answer[apigen.ListExecutionData200JSONResponse](t, data).Body)
	if err != nil {
		t.Fatal(err)
	}
	// Decode and encode again without the HTML escaping, so the page text is readable.
	var doc any
	if err := json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	var raw strings.Builder
	enc := json.NewEncoder(&raw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		t.Fatal(err)
	}
	text := raw.String()
	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Fatalf("the served data lacks %q: %s", w, text)
		}
	}
}
