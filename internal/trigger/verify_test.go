// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger_test

import (
	"context"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/trigger"
	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

// routedStore answers every route with one fixed address and keeps the
// in-memory store for everything else.
type routedStore struct {
	*triggertest.Memory
	route trigger.Resolved
}

func (s routedStore) ResolveRoute(_ context.Context, _, _ string) (trigger.Resolved, error) {
	return s.route, nil
}

// fakeVerifier records the requests it sees and answers with a fixed outcome.
type fakeVerifier struct {
	verifyErr    error
	challenge    string
	challengeErr error
	verified     int
}

func (v *fakeVerifier) Verify(context.Context, trigger.Resolved, trigger.IngressRequest) error {
	v.verified++
	return v.verifyErr
}

func (v *fakeVerifier) Challenge(context.Context, trigger.Resolved, trigger.IngressRequest) (string, error) {
	return v.challenge, v.challengeErr
}

func newIngress(t *testing.T, v trigger.Verifier) (*trigger.Service, *triggertest.RecordingStarter) {
	t.Helper()
	mem := triggertest.NewMemory()
	starter := &triggertest.RecordingStarter{}
	svc, err := trigger.New(trigger.Config{
		Store:       routedStore{Memory: mem},
		Flows:       mem,
		Catalog:     triggertest.DefaultCatalog(),
		Credentials: &triggertest.Credentials{},
		Starter:     starter,
		Clock:       clock.NewFake(triggertest.Start),
		IDs:         id.NewSequence(),
		Verifier:    v,
	})
	if err != nil {
		t.Fatalf("trigger.New: %v", err)
	}
	return svc, starter
}

// TestIngestRefusesAnUnverifiedRequestBeforeAnyExecution: a request the
// verifier refuses is answered as unauthenticated and starts nothing.
func TestIngestRefusesAnUnverifiedRequestBeforeAnyExecution(t *testing.T) {
	v := &fakeVerifier{verifyErr: errors.New("signature does not match")}
	svc, starter := newIngress(t, v)

	_, err := svc.Ingest(t.Context(), trigger.IngressRequest{Method: "POST", Path: "/hooks/p/x", Body: []byte(`{}`)})
	if !errors.Is(err, trigger.ErrUnauthorized) {
		t.Fatalf("Ingest() = %v, want ErrUnauthorized", err)
	}
	if v.verified != 1 {
		t.Fatalf("verifier saw %d requests, want 1", v.verified)
	}
	if len(starter.Triggered) != 0 {
		t.Fatalf("%d executions started for a refused request", len(starter.Triggered))
	}
}

// TestHandshakeEchoesTheVerifiersChallenge: the subscription answer is the
// challenge the verifier accepted.
func TestHandshakeEchoesTheVerifiersChallenge(t *testing.T) {
	svc, starter := newIngress(t, &fakeVerifier{challenge: "1158201444"})
	got, err := svc.Handshake(t.Context(), trigger.IngressRequest{Method: "GET", Path: "/hooks/p/x"})
	if err != nil || got != "1158201444" {
		t.Fatalf("Handshake() = %q, %v; want the challenge", got, err)
	}
	if len(starter.Triggered) != 0 {
		t.Fatal("a handshake started an execution")
	}
}

// TestHandshakeRefusedWithoutAVerifier: a build that cannot check the verify
// token must not confirm an address.
func TestHandshakeRefusedWithoutAVerifier(t *testing.T) {
	svc, _ := newIngress(t, nil)
	if _, err := svc.Handshake(t.Context(), trigger.IngressRequest{Method: "GET", Path: "/hooks/p/x"}); !errors.Is(err, trigger.ErrUnauthorized) {
		t.Fatalf("Handshake() = %v, want ErrUnauthorized", err)
	}
}

// TestHandshakeRefusedForAWrongToken: a refused challenge is unauthenticated.
func TestHandshakeRefusedForAWrongToken(t *testing.T) {
	svc, _ := newIngress(t, &fakeVerifier{challengeErr: errors.New("handshake refused")})
	if _, err := svc.Handshake(t.Context(), trigger.IngressRequest{Method: "GET", Path: "/hooks/p/x"}); !errors.Is(err, trigger.ErrUnauthorized) {
		t.Fatalf("Handshake() = %v, want ErrUnauthorized", err)
	}
}
