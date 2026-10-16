// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"errors"
	"testing"
)

// metaTextEvent is a text message in the shape the Graph API sends to a
// webhook (object, entry, changes, value, messages). Its signature under the
// secret "test-app-secret" was computed with an independent tool (openssl
// dgst -sha256 -hmac) and is fixed here.
const metaTextEvent = `{"object":"whatsapp_business_account","entry":[{"id":"102290129340398","changes":[{"value":{"messaging_product":"whatsapp","metadata":{"display_phone_number":"15550783881","phone_number_id":"106540352242922"},"contacts":[{"profile":{"name":"Sheena Nelson"},"wa_id":"16505551234"}],"messages":[{"from":"16505551234","id":"wamid.HBgLMTY1MDM4Nzg4ODAVAgASGBQzQTU2RjM1NjdGRjE5NEZDODg2RQA=","timestamp":"1504902988","type":"text","text":{"body":"Does it come in another color?"}}]},"field":"messages"}]}]}`

const (
	metaSecret          = "test-app-secret"
	metaSignature       = "sha256=90224e6cd66be90228db3850b2c941fec5482f2f4d90b0b0972b5cf22c2f2092"
	metaSignatureHeader = "X-Hub-Signature-256"
)

var metaVerify = Verify{Type: VerifyHMACSHA256, Header: metaSignatureHeader, Prefix: "sha256=", Field: "appSecret"}

func TestVerifySignatureAcceptsTheMetaDigest(t *testing.T) {
	header := map[string][]string{metaSignatureHeader: {metaSignature}}
	if err := VerifySignature(metaVerify, header, []byte(metaTextEvent), metaSecret); err != nil {
		t.Fatalf("VerifySignature() refused the provider's own digest: %v", err)
	}
}

func TestVerifySignatureRefusesEveryMismatch(t *testing.T) {
	good := map[string][]string{metaSignatureHeader: {metaSignature}}
	tampered := metaTextEvent[:len(metaTextEvent)-2] + `"}`
	cases := []struct {
		name   string
		header map[string][]string
		body   []byte
		secret string
		verify Verify
	}{
		{"wrong secret", good, []byte(metaTextEvent), "another-secret", metaVerify},
		{"changed body", good, []byte(tampered), metaSecret, metaVerify},
		{"header missing", map[string][]string{}, []byte(metaTextEvent), metaSecret, metaVerify},
		{"prefix missing", map[string][]string{metaSignatureHeader: {metaSignature[len("sha256="):]}}, []byte(metaTextEvent), metaSecret, metaVerify},
		{"not hex", map[string][]string{metaSignatureHeader: {"sha256=zz"}}, []byte(metaTextEvent), metaSecret, metaVerify},
		{"empty secret never matches", good, []byte(metaTextEvent), "", metaVerify},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := VerifySignature(c.verify, c.header, c.body, c.secret)
			if !errors.Is(err, ErrSignature) {
				t.Fatalf("VerifySignature() = %v, want ErrSignature", err)
			}
		})
	}
}

// TestVerifySignatureHeaderNameIsCaseInsensitive: transports canonicalize
// header names differently, so the lookup must not depend on their case.
func TestVerifySignatureHeaderNameIsCaseInsensitive(t *testing.T) {
	header := map[string][]string{"x-hub-signature-256": {metaSignature}}
	if err := VerifySignature(metaVerify, header, []byte(metaTextEvent), metaSecret); err != nil {
		t.Fatalf("lower-case header refused: %v", err)
	}
}

// TestVerifySignatureWithoutPrefix covers a provider that sends the bare hex
// digest in its header.
func TestVerifySignatureWithoutPrefix(t *testing.T) {
	v := Verify{Type: VerifyHMACSHA256, Header: "X-Signature", Field: "secret"}
	header := map[string][]string{"X-Signature": {metaSignature[len("sha256="):]}}
	if err := VerifySignature(v, header, []byte(metaTextEvent), metaSecret); err != nil {
		t.Fatalf("bare digest refused: %v", err)
	}
}

func TestCheckHandshakeEchoesTheChallenge(t *testing.T) {
	h := Handshake{Type: HandshakeEcho, ModeParam: "hub.mode", TokenParam: "hub.verify_token", ChallengeParam: "hub.challenge", Field: "verifyToken"}
	query := map[string]string{"hub.mode": "subscribe", "hub.verify_token": "orcon-verify", "hub.challenge": "1158201444"}
	got, err := CheckHandshake(h, query, "orcon-verify")
	if err != nil {
		t.Fatalf("CheckHandshake() refused a valid subscription: %v", err)
	}
	if got != "1158201444" {
		t.Fatalf("challenge = %q, want the echoed value", got)
	}
}

func TestCheckHandshakeRefusesWhatIsNotASubscription(t *testing.T) {
	h := Handshake{Type: HandshakeEcho, ModeParam: "hub.mode", TokenParam: "hub.verify_token", ChallengeParam: "hub.challenge", Field: "verifyToken"}
	valid := func() map[string]string {
		return map[string]string{"hub.mode": "subscribe", "hub.verify_token": "orcon-verify", "hub.challenge": "42"}
	}
	cases := map[string]struct {
		query map[string]string
		token string
	}{
		"wrong token":       {valid(), "other"},
		"no verify token":   {valid(), ""},
		"unsubscribe mode":  {map[string]string{"hub.mode": "unsubscribe", "hub.verify_token": "orcon-verify", "hub.challenge": "42"}, "orcon-verify"},
		"missing mode":      {map[string]string{"hub.verify_token": "orcon-verify", "hub.challenge": "42"}, "orcon-verify"},
		"missing challenge": {map[string]string{"hub.mode": "subscribe", "hub.verify_token": "orcon-verify"}, "orcon-verify"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := CheckHandshake(h, c.query, c.token); !errors.Is(err, ErrHandshake) {
				t.Fatalf("CheckHandshake() = %v, want ErrHandshake", err)
			}
		})
	}
}

func TestValidateTriggers(t *testing.T) {
	webhook := Trigger{
		Family: FamilyWebhook,
		Verify: &Verify{Type: VerifyHMACSHA256, Header: metaSignatureHeader, Prefix: "sha256=", Field: "appSecret"},
		Handshake: &Handshake{Type: HandshakeEcho, ModeParam: "hub.mode", TokenParam: "hub.verify_token",
			ChallengeParam: "hub.challenge", Field: "verifyToken"},
	}
	build := func(t Trigger) Definition {
		def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": readOp("/items")})
		def.Triggers = map[string]Trigger{"inbound": t}
		return def
	}
	if err := Validate(build(webhook)); err != nil {
		t.Fatalf("a complete webhook trigger was refused: %v", err)
	}

	cases := map[string]func(*Trigger){
		"poll family is not run yet": func(t *Trigger) { t.Family = "POLL" },
		"neither verify nor handshake": func(t *Trigger) {
			t.Verify, t.Handshake = nil, nil
		},
		"verify without a secret field": func(t *Trigger) { t.Verify.Field = "" },
		"verify with another algorithm": func(t *Trigger) { t.Verify.Type = "MD5" },
		"verify without a header":       func(t *Trigger) { t.Verify.Header = "" },
		"handshake with another type":   func(t *Trigger) { t.Handshake.Type = "SLACK_URL" },
		"handshake without parameters":  func(t *Trigger) { t.Handshake.TokenParam = "" },
		"handshake without a field":     func(t *Trigger) { t.Handshake.Field = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			tr := webhook
			verify, hs := *webhook.Verify, *webhook.Handshake
			tr.Verify, tr.Handshake = &verify, &hs
			mutate(&tr)
			if err := Validate(build(tr)); err == nil {
				t.Fatal("Validate accepted the trigger")
			}
		})
	}
}
