// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/egress"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/trigger"
	"github.com/wirvii/orcon/test/mockllm"
	"github.com/wirvii/orcon/test/mockprovider"
)

// The fixed values of the end-to-end test: the signing secret and the
// subscription token of the WhatsApp credential, and the sender of the messages.
const (
	testAppSecret    = "test-app-secret"
	testVerifyToken  = "test-verify-token"
	testSender       = "573000000000"
	whatsappFlowFile = "../../examples/flows/whatsapp-agent.json"
	whatsappRoute    = "/hooks/engine/whatsapp"
	graphHost        = "https://graph.facebook.com"

	// The placeholders of the example flow for the identifiers of the credentials an owner stores.
	whatsappCredentialPlaceholder = "REPLACE-WITH-THE-WHATSAPP-CREDENTIAL-ID"
	modelCredentialPlaceholder    = "REPLACE-WITH-THE-MODEL-CREDENTIAL-ID"
)

// TestWhatsAppMessageRunsAgentAndReplies is the first end-to-end run of the
// agent flow (SPEC-003 WhatsApp inbound and outbound, SPEC-006 agent node): a
// signed WhatsApp notification starts the example flow, the agent answers
// through the scripted model with the memory of the sender, and the reply goes
// to the Graph endpoint of the mock provider. Both stores run the same test.
func TestWhatsAppMessageRunsAgentAndReplies(t *testing.T) {
	for _, kind := range e2eKinds() {
		t.Run(string(kind), func(t *testing.T) {
			runWhatsAppAgentFlow(t, kind)
		})
	}
}

func runWhatsAppAgentFlow(t *testing.T, kind storetest.Kind) {
	t.Helper()
	ctx := t.Context()
	env := storetest.NewEngineEnv(t, kind, storetest.EngineOptions{})
	st, ok := env.Store.(engineStore)
	if !ok {
		t.Fatalf("the %s store does not implement the engine store", kind)
	}

	graph := mockprovider.New()
	graphSrv := httptest.NewServer(graph)
	t.Cleanup(graphSrv.Close)
	graphURL, err := url.Parse(graphSrv.URL)
	if err != nil {
		t.Fatalf("parse the Graph URL: %v", err)
	}

	// The three secrets of the flow are stored credentials of its project, bound to the host each one is sent to:
	// the WhatsApp one carries the bearer token of the sends and the secrets that check the inbound requests,
	// the model one carries the API key.
	svc, owner := credentialServiceOf(t, env)
	whatsappID := storeCredential(t, svc, env, owner, "WhatsApp", "whatsapp", graphURL.Hostname(), map[string]string{
		"token": mockprovider.Token, "appSecret": testAppSecret, "verifyToken": testVerifyToken,
	})
	openAIID := storeCredential(t, svc, env, owner, "OpenAI", "openai", openAIHost, map[string]string{"apiKey": "test-key"})

	llm := mockllm.New(t,
		mockllm.Reply{Body: chatCompletion("Tu pedido sale hoy.")},
		mockllm.Reply{Body: chatCompletion("Si no estas, lo dejamos con el vecino.")},
	)

	eng, err := newEngine(st, engineOptions{
		role: config.RoleStandalone, logger: slog.New(slog.DiscardHandler), version: "test",
		senderPolicy: func(u *url.URL) egress.Policy {
			return egress.Policy{Domains: []string{u.Hostname()}, Schemes: []string{"http"}, AllowPrivateNetworks: true}
		},
		resolver:      svc,
		modelEndpoint: &modelEndpoint{baseURL: llm.URL(), client: llm.Client()},
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

	publishWhatsAppFlow(t, env, eng, flowCredentials{graphURL: graphSrv.URL, whatsapp: whatsappID, openAI: openAIID})

	handler, err := newAPIRouterWith(config.RoleStandalone, slog.New(slog.DiscardHandler), nil, eng.ingress())
	if err != nil {
		t.Fatalf("build the router: %v", err)
	}

	// The subscription handshake echoes the challenge when the token matches.
	handshake(t, handler, testVerifyToken, "1158201444", http.StatusOK, "1158201444")
	handshake(t, handler, "wrong-token", "1158201445", http.StatusForbidden, "")

	// The first message starts one execution; the agent answers and the reply is sent.
	exec := postSigned(t, handler, messageBody("wamid.1", "When does my order arrive?"), testAppSecret, http.StatusAccepted)
	waitExecution(t, env, st, exec)

	// The second message comes from the same sender, so the model sees the first exchange.
	exec = postSigned(t, handler, messageBody("wamid.2", "And if I am not home?"), testAppSecret, http.StatusAccepted)
	waitExecution(t, env, st, exec)

	model := llm.Requests()
	if len(model) != 2 {
		t.Fatalf("the model received %d requests, want 2", len(model))
	}
	if !strings.Contains(string(model[1].Body), "When does my order arrive?") {
		t.Fatalf("the second model call does not carry the first message of the sender: %s", model[1].Body)
	}

	sent := graph.Requests()
	if len(sent) != 2 {
		t.Fatalf("the Graph endpoint received %d sends, want 2", len(sent))
	}
	for i, answer := range []string{"Tu pedido sale hoy.", "Si no estas, lo dejamos con el vecino."} {
		r := sent[i]
		if r.Method != http.MethodPost || r.Path != mockprovider.GraphPrefix+mockprovider.MockPhoneNumberID+"/messages" {
			t.Fatalf("send %d went to %s %s", i, r.Method, r.Path)
		}
		if r.Authorization != "Bearer "+mockprovider.Token {
			t.Fatalf("send %d carried Authorization %q", i, r.Authorization)
		}
		body := string(r.Body)
		if !strings.Contains(body, testSender) || !strings.Contains(body, answer) {
			t.Fatalf("send %d body %s does not carry the recipient and the answer %q", i, body, answer)
		}
	}

	// A notification that carries no message (a delivery status) starts nothing.
	status := []byte(`{"entry":[{"changes":[{"value":{"statuses":[{"id":"wamid.1","status":"delivered"}]}}]}]}`)
	postSigned(t, handler, string(status), testAppSecret, http.StatusOK)
	if n := len(llm.Requests()); n != 2 {
		t.Fatalf("a status notification reached the model: %d requests", n)
	}

	// A request with a wrong signature is refused before anything starts.
	postSigned(t, handler, messageBody("wamid.3", "forged"), "not-the-secret", http.StatusUnauthorized)
	if n := len(llm.Requests()); n != 2 {
		t.Fatalf("a forged request reached the model: %d requests", n)
	}
}

// flowCredentials are what an owner fills in after importing the example flow: the
// address of the Graph endpoint and the identifiers of the two stored credentials.
type flowCredentials struct {
	graphURL string
	whatsapp string
	openAI   string
}

// publishWhatsAppFlow stores the example flow, points its Graph address at the
// mock provider and its credential placeholders at the stored credentials,
// publishes it and activates it, through the trigger service the process runs.
// The example itself is what an owner imports; only those values change.
func publishWhatsAppFlow(t *testing.T, env *storetest.EngineEnv, eng *engine, fill flowCredentials) {
	t.Helper()
	ctx := t.Context()
	raw, err := os.ReadFile(whatsappFlowFile)
	if err != nil {
		t.Fatalf("read the example flow: %v", err)
	}
	draft := string(raw)
	for placeholder, value := range map[string]string{
		graphHost: fill.graphURL, whatsappCredentialPlaceholder: fill.whatsapp, modelCredentialPlaceholder: fill.openAI,
	} {
		if !strings.Contains(draft, placeholder) {
			t.Fatalf("the example flow has no %q to fill in", placeholder)
		}
		draft = strings.ReplaceAll(draft, placeholder, value)
	}
	now := env.Clock.Now()
	rec := flow.Record{
		ID: flow.FlowID(env.IDs.New()), OrgID: env.Org, ProjectID: env.Project, Name: "whatsapp-agent",
		Draft: []byte(draft), CreatedAt: now, UpdatedAt: now,
	}
	org := storetest.ScopeOf(env.Org)
	if err := env.Store.CreateFlow(ctx, org, rec); err != nil {
		t.Fatalf("create flow: %v", err)
	}
	by := identity.PrincipalID(env.IDs.New())
	if err := env.Store.CreatePrincipal(ctx, identity.PrincipalRecord{ID: by, Kind: identity.PrincipalUser, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("create the publishing principal: %v", err)
	}
	if _, err := eng.trigger.Publish(ctx, trigger.PublishCommand{Org: org, Flow: rec.ID, By: by}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := eng.trigger.Activate(ctx, org, rec.ID); err != nil {
		t.Fatalf("activate: %v", err)
	}
}

// handshake sends the subscription GET of the provider and checks the status and the echo.
func handshake(t *testing.T, h http.Handler, token, challenge string, want int, echo string) {
	t.Helper()
	target := whatsappRoute + "?hub.mode=subscribe&hub.verify_token=" + url.QueryEscape(token) + "&hub.challenge=" + challenge
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("GET handshake with token %q: status %d, want %d: %s", token, rec.Code, want, rec.Body.String())
	}
	if want == http.StatusOK && rec.Body.String() != echo {
		t.Fatalf("handshake echoed %q, want %q", rec.Body.String(), echo)
	}
}

// postSigned sends one notification signed with secret, as the provider signs it,
// and checks the status. For a 202 it returns the execution the request started.
func postSigned(t *testing.T, h http.Handler, body, secret string, want int) store.ExecutionID {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, whatsappRoute, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("POST %s: status %d, want %d: %s", whatsappRoute, rec.Code, want, rec.Body.String())
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

// messageBody is one Meta notification carrying one text message from the test sender.
func messageBody(messageID, text string) string {
	return fmt.Sprintf(`{"object":"whatsapp_business_account","entry":[{"id":"waba-1","changes":[{"field":"messages","value":{`+
		`"messaging_product":"whatsapp","metadata":{"display_phone_number":"15550100","phone_number_id":%q},`+
		`"contacts":[{"profile":{"name":"Ana"},"wa_id":%q}],`+
		`"messages":[{"from":%q,"id":%q,"timestamp":"1760000000","type":"text","text":{"body":%q}}]}}]}]}`,
		mockprovider.MockPhoneNumberID, testSender, testSender, messageID, text)
}

// chatCompletion is one OpenAI chat answer with the given text.
func chatCompletion(text string) string {
	return fmt.Sprintf(`{"id":"chatcmpl-test","object":"chat.completion","model":"gpt-4o-mini",`+
		`"choices":[{"index":0,"message":{"role":"assistant","content":%q,"refusal":null},"finish_reason":"stop"}],`+
		`"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`, text)
}
