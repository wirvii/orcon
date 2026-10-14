// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package conformancetest

import (
	"context"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func definition() declarative.Definition {
	return declarative.Definition{
		Format: declarative.Format,
		HTTP:   declarative.HTTP{BaseURL: "https://api.example.com", Auth: declarative.Auth{Type: declarative.AuthBearer}},
		Operations: map[string]declarative.Operation{
			"item.get": {
				Purity: declarative.PurityRead, Safety: declarative.SafetyReadOnly,
				Request:  declarative.Request{Method: "GET", Path: "/items/{{$parameter.id}}"},
				Response: declarative.ResponseSpec{Select: "$.data"},
			},
		},
	}
}

func TestRunCassetteReplaysRecordedExchange(t *testing.T) {
	in, err := declarative.New(definition(), declarative.Options{Sender: noSender{}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	item, err := jsonvalue.Parse([]byte(`{"id":"7"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	RunCassette(t, in, Cassette{
		Name:        "get one item",
		Operation:   "item.get",
		Parameters:  map[string]jsonvalue.Value{"id": jsonvalue.String("7")},
		Credentials: map[string]string{"token": "tok-secret"},
		Interactions: []Interaction{{
			Method:   "GET",
			URL:      "https://api.example.com/items/7",
			Header:   map[string]string{"Authorization": "Bearer tok-secret"},
			Response: declarative.HTTPResponse{Status: 200, Body: []byte(`{"data":{"id":"7"}}`)},
		}},
		Items: []jsonvalue.Value{item},
	})
}

// noSender is replaced by the replay sender inside RunCassette.
type noSender struct{}

func (noSender) Send(_ context.Context, _ declarative.HTTPRequest) (declarative.HTTPResponse, error) {
	return declarative.HTTPResponse{}, errors.New("not replayed")
}
