// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package egress_test

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/egress"
	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/test/mockprovider"
)

// harness runs the declarative interpreter through the real egress client
// against the simulated provider over loopback HTTP.
type harness struct {
	provider *mockprovider.Provider
	server   *httptest.Server
	client   *egress.Client
	policy   egress.Policy
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	p := mockprovider.New()
	srv := httptest.NewServer(p)
	t.Cleanup(srv.Close)
	client, err := egress.New(egress.Config{})
	if err != nil {
		t.Fatalf("egress.New: %v", err)
	}
	return &harness{
		provider: p,
		server:   srv,
		client:   client,
		policy: egress.Policy{
			Domains:              []string{"127.0.0.1"},
			Schemes:              []string{"http"},
			AllowPrivateNetworks: true,
		},
	}
}

func (h *harness) interpreter(t *testing.T, ops map[string]declarative.Operation) *declarative.Interpreter {
	t.Helper()
	def := declarative.Definition{
		Format: declarative.Format,
		HTTP: declarative.HTTP{
			BaseURL: h.server.URL + "/v1",
			Auth:    declarative.Auth{Type: declarative.AuthBearer},
		},
		Operations: ops,
	}
	in, err := declarative.New(def, declarative.Options{Sender: h.client.Sender(h.policy)})
	if err != nil {
		t.Fatalf("declarative.New: %v", err)
	}
	return in
}

var creds = map[string]string{"token": mockprovider.Token}

func TestDeclarativeOffsetPagingOverLoopback(t *testing.T) {
	h := newHarness(t)
	in := h.interpreter(t, map[string]declarative.Operation{
		"item.list": {
			Purity: declarative.PurityRead, Safety: declarative.SafetyReadOnly,
			Request:  declarative.Request{Method: "GET", Path: "/items"},
			Response: declarative.ResponseSpec{Select: "$.data"},
			Pagination: &declarative.Pagination{
				Strategy: "OFFSET", Param: "offset", LimitParam: "limit", PageSize: 10,
			},
		},
	})
	res, err := in.Execute(t.Context(), declarative.Call{Operation: "item.list", Credentials: creds}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Items) != mockprovider.ItemCount || res.Pages != 3 {
		t.Fatalf("items=%d pages=%d", len(res.Items), res.Pages)
	}
	for _, r := range h.provider.Requests() {
		if r.Authorization != "Bearer "+mockprovider.Token {
			t.Fatalf("request without the credential: %+v", r)
		}
	}
}

func TestDeclarativeRejectsWrongCredential(t *testing.T) {
	h := newHarness(t)
	in := h.interpreter(t, map[string]declarative.Operation{
		"item.list": {
			Purity: declarative.PurityRead, Safety: declarative.SafetyReadOnly,
			Request: declarative.Request{Method: "GET", Path: "/items"},
		},
	})
	_, err := in.Execute(t.Context(), declarative.Call{Operation: "item.list", Credentials: map[string]string{"token": "wrong"}}, nil)
	var f *declarative.CallError
	if !errors.As(err, &f) || f.Class != declarative.ClassPermanent || f.Code != "AuthInvalid" {
		t.Fatalf("err = %v", err)
	}
}

func TestDeclarativeUnsafeWriteCutAfterSendIsUncertainAndNotResent(t *testing.T) {
	h := newHarness(t)
	in := h.interpreter(t, map[string]declarative.Operation{
		"thing.create": {
			Purity: declarative.PurityEffect, Safety: declarative.SafetyUnsafe,
			Request: declarative.Request{
				Method:  "POST",
				Path:    "/things",
				Headers: map[string]string{"X-Mock-Fault": "hangup"},
			},
		},
	})
	_, err := in.Execute(t.Context(), declarative.Call{Operation: "thing.create", Credentials: creds}, nil)
	var f *declarative.CallError
	if !errors.As(err, &f) || f.Class != declarative.ClassUncertain || f.Action != declarative.ActionSuspend {
		t.Fatalf("err = %v", err)
	}
	if n := len(h.provider.Requests()); n != 1 {
		t.Fatalf("the provider received %d requests, want exactly 1", n)
	}
	if n := h.provider.AppliedWrites(""); n != 1 {
		t.Fatalf("the write was applied %d times, want 1", n)
	}
}

func TestDeclarativeDeniedHostIsPermanentAndNotSent(t *testing.T) {
	h := newHarness(t)
	def := declarative.Definition{
		Format: declarative.Format,
		HTTP: declarative.HTTP{
			BaseURL: "http://localhost" + h.server.Listener.Addr().String()[len("127.0.0.1"):] + "/v1",
			Auth:    declarative.Auth{Type: declarative.AuthNone},
		},
		Operations: map[string]declarative.Operation{
			"item.list": {
				Purity: declarative.PurityRead, Safety: declarative.SafetyReadOnly,
				Request: declarative.Request{Method: "GET", Path: "/items"},
			},
		},
	}
	in, err := declarative.New(def, declarative.Options{Sender: h.client.Sender(h.policy)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = in.Execute(t.Context(), declarative.Call{Operation: "item.list"}, nil)
	var f *declarative.CallError
	if !errors.As(err, &f) || f.Code != declarative.CodeEgressDenied || f.Class != declarative.ClassPermanent {
		t.Fatalf("err = %v", err)
	}
	if n := len(h.provider.Requests()); n != 0 {
		t.Fatalf("a refused destination received %d requests", n)
	}
}

func TestDeclarativeParameterReachesBody(t *testing.T) {
	h := newHarness(t)
	in := h.interpreter(t, map[string]declarative.Operation{
		"thing.create": {
			Purity: declarative.PurityEffect, Safety: declarative.SafetyIdempotencyKey,
			IdempotencyHeader: "Idempotency-Key",
			IdempotencyKey:    "{{$parameter.key}}",
			Request: declarative.Request{
				Method: "POST",
				Path:   "/things",
				Body: &declarative.Body{Type: "JSON", Fields: map[string]declarative.Node{
					"name": {Value: jsonvalue.String("{{$parameter.name}}")},
				}},
			},
		},
	})
	params := map[string]jsonvalue.Value{"key": jsonvalue.String("k-9"), "name": jsonvalue.String("alpha")}
	res, err := in.Execute(t.Context(), declarative.Call{Operation: "thing.create", Parameters: params, Credentials: creds}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("items = %d, want the single response object", len(res.Items))
	}
	if n := h.provider.AppliedWrites("k-9"); n != 1 {
		t.Fatalf("applied %d times, want 1", n)
	}
}
