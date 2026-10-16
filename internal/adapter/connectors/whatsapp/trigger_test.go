// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package whatsapp_test

import (
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/wirvii/orcon/internal/connector/declarative"
)

// The fixture is a text message as the Graph API sends it to a webhook. Its
// signature under the secret "test-app-secret" is the digest that openssl
// computes for the same bytes; the test does not compute it itself.
const (
	fixtureSecret       = "test-app-secret"
	fixtureSignature    = "sha256=90224e6cd66be90228db3850b2c941fec5482f2f4d90b0b0972b5cf22c2f2092"
	fixtureHandshakeKey = "orcon-verify-value"
)

func loadFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/webhook-text-message.json")
	if err != nil {
		t.Fatalf("reading the webhook fixture: %v", err)
	}
	return data
}

// TestInboundTriggerIsDeclared: the connector declares one webhook trigger
// whose verification is the HMAC-SHA256 scheme of Meta, and whose handshake
// echoes the challenge for the verify token.
func TestInboundTriggerIsDeclared(t *testing.T) {
	def := loadDefinition(t)
	tr, ok := def.Triggers["whatsappTrigger"]
	if !ok {
		t.Fatal("the connector declares no inbound trigger")
	}
	if tr.Family != declarative.FamilyWebhook {
		t.Fatalf("family = %s, want WEBHOOK", tr.Family)
	}
	if tr.Verify == nil || tr.Verify.Type != declarative.VerifyHMACSHA256 || tr.Verify.Header != "X-Hub-Signature-256" || tr.Verify.Prefix != "sha256=" {
		t.Fatalf("verify = %+v, want HMAC_SHA256 over X-Hub-Signature-256 with sha256=", tr.Verify)
	}
	if tr.Handshake == nil || tr.Handshake.ModeParam != "hub.mode" || tr.Handshake.TokenParam != "hub.verify_token" || tr.Handshake.ChallengeParam != "hub.challenge" {
		t.Fatalf("handshake = %+v, want the hub.* query parameters", tr.Handshake)
	}
}

// TestInboundSignatureOfAMetaEvent: the signature of a real event is accepted
// under the app secret and refused under any other secret or changed bytes.
func TestInboundSignatureOfAMetaEvent(t *testing.T) {
	tr := loadDefinition(t).Triggers["whatsappTrigger"]
	body := loadFixture(t)
	header := map[string][]string{"X-Hub-Signature-256": {fixtureSignature}}

	if err := declarative.VerifySignature(*tr.Verify, header, body, fixtureSecret); err != nil {
		t.Fatalf("the event of the provider was refused: %v", err)
	}
	if err := declarative.VerifySignature(*tr.Verify, header, body, "other-secret"); !errors.Is(err, declarative.ErrSignature) {
		t.Fatalf("a wrong secret was accepted: %v", err)
	}
	changed := append(append([]byte{}, body[:len(body)-1]...), ' ', '}')
	if err := declarative.VerifySignature(*tr.Verify, header, changed, fixtureSecret); !errors.Is(err, declarative.ErrSignature) {
		t.Fatalf("a changed body was accepted: %v", err)
	}
}

// TestInboundSubscriptionHandshake: Meta sends the subscription GET with
// hub.mode, hub.verify_token and hub.challenge; the challenge is echoed only
// for the right token.
func TestInboundSubscriptionHandshake(t *testing.T) {
	tr := loadDefinition(t).Triggers["whatsappTrigger"]
	query := map[string]string{"hub.mode": "subscribe", "hub.verify_token": fixtureHandshakeKey, "hub.challenge": "1158201444"}

	got, err := declarative.CheckHandshake(*tr.Handshake, query, fixtureHandshakeKey)
	if err != nil || got != "1158201444" {
		t.Fatalf("CheckHandshake() = %q, %v; want the challenge echoed", got, err)
	}
	if _, err := declarative.CheckHandshake(*tr.Handshake, query, "another-token"); !errors.Is(err, declarative.ErrHandshake) {
		t.Fatalf("a wrong verify token was accepted: %v", err)
	}
}

// TestInboundSignatureHeaderIsReadWhateverItsCase: the header arrives from the
// transport with its canonical or its lower-case name.
func TestInboundSignatureHeaderIsReadWhateverItsCase(t *testing.T) {
	tr := loadDefinition(t).Triggers["whatsappTrigger"]
	header := http.Header{}
	header.Set("x-hub-signature-256", fixtureSignature)
	if err := declarative.VerifySignature(*tr.Verify, header, loadFixture(t), fixtureSecret); err != nil {
		t.Fatalf("lower-case header refused: %v", err)
	}
}
