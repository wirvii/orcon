// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/trigger"
)

// fakeWebhookIngress records what reaches the ingress and answers with fixed
// outcomes.
type fakeWebhookIngress struct {
	ingestErr    error
	challenge    string
	handshakeErr error
	got          trigger.IngressRequest
}

func (f *fakeWebhookIngress) Ingest(_ context.Context, req trigger.IngressRequest) (trigger.Accepted, error) {
	f.got = req
	if f.ingestErr != nil {
		return trigger.Accepted{}, f.ingestErr
	}
	return trigger.Accepted{ExecutionID: "exec-1"}, nil
}

func (f *fakeWebhookIngress) Handshake(_ context.Context, req trigger.IngressRequest) (string, error) {
	f.got = req
	return f.challenge, f.handshakeErr
}

func serveWebhook(in WebhookIngress, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	webhookHandler(in, slog.New(slog.DiscardHandler)).ServeHTTP(rec, r)
	return rec
}

// TestWebhookPassesTheSignatureHeaderToTheIngress: the verifier needs the raw
// body and the headers as they arrived.
func TestWebhookPassesTheSignatureHeaderToTheIngress(t *testing.T) {
	in := &fakeWebhookIngress{}
	body := `{"object":"whatsapp_business_account"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hooks/p/inbound", strings.NewReader(body))
	r.Header.Set("X-Hub-Signature-256", "sha256=abc")

	rec := serveWebhook(in, r)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if got := in.got.Headers["X-Hub-Signature-256"]; len(got) != 1 || got[0] != "sha256=abc" {
		t.Fatalf("headers reaching the ingress = %v", in.got.Headers)
	}
	if string(in.got.Body) != body {
		t.Fatalf("body reaching the ingress = %q", in.got.Body)
	}
}

// TestWebhookAnswersAnUnauthenticatedRequestAs401: a POST whose signature is
// refused gets the unauthenticated problem and no execution id.
func TestWebhookAnswersAnUnauthenticatedRequestAs401(t *testing.T) {
	in := &fakeWebhookIngress{ingestErr: trigger.ErrUnauthorized}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/hooks/p/inbound", strings.NewReader(`{}`))
	rec := serveWebhook(in, r)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if rec.Header().Get("Orcon-Execution-Id") != "" {
		t.Fatal("a refused request carries an execution id")
	}
}

// TestWebhookGetIsTheSubscriptionHandshake: a GET is answered with the
// challenge as plain text, and its query reaches the ingress.
func TestWebhookGetIsTheSubscriptionHandshake(t *testing.T) {
	in := &fakeWebhookIngress{challenge: "1158201444"}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hooks/p/inbound?hub.mode=subscribe&hub.verify_token=t&hub.challenge=1158201444", nil)

	rec := serveWebhook(in, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "1158201444" {
		t.Fatalf("body = %q, want the challenge alone", body)
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("content type = %q, want text/plain", rec.Header().Get("Content-Type"))
	}
	if in.got.Query["hub.mode"] != "subscribe" || in.got.Query["hub.verify_token"] != "t" {
		t.Fatalf("query reaching the ingress = %v", in.got.Query)
	}
}

// TestWebhookRefusedHandshakeIs403: a wrong token is forbidden and says
// nothing about the address.
func TestWebhookRefusedHandshakeIs403(t *testing.T) {
	in := &fakeWebhookIngress{handshakeErr: errors.Join(trigger.ErrUnauthorized, errors.New("token"))}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hooks/p/inbound?hub.mode=subscribe&hub.verify_token=bad&hub.challenge=1", nil)
	rec := serveWebhook(in, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if rec.Header().Get("Content-Type") == "text/plain; charset=utf-8" {
		t.Fatal("a refused handshake answered as a challenge")
	}
}
