// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func mustParse(t *testing.T, s string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(s))
	if err != nil {
		t.Fatalf("jsonvalue.Parse(%s): %v", s, err)
	}
	return v
}

const orderSchema = `{
  "type": "object",
  "properties": {
    "sku": {"type": "string"},
    "quantity": {"type": "integer"},
    "priority": {"enum": ["LOW", "HIGH"]},
    "items": {"type": "array", "items": {"type": "object", "properties": {"id": {"type": "string"}}, "required": ["id"]}}
  },
  "required": ["sku", "quantity"],
  "additionalProperties": false
}`

func TestArgumentValidation(t *testing.T) {
	t.Parallel()
	schema := mustParse(t, orderSchema)
	tests := []struct {
		name      string
		args      string
		wantPath  string // empty when the arguments must validate
		wantError bool
	}{
		{name: "valid arguments", args: `{"sku":"A-1","quantity":2}`},
		{name: "optional properties may be absent", args: `{"sku":"A-1","quantity":2,"priority":"HIGH"}`},
		{name: "missing required property", args: `{"sku":"A-1"}`, wantPath: "$", wantError: true},
		{name: "wrong type for a property", args: `{"sku":"A-1","quantity":"2"}`, wantPath: "$.quantity", wantError: true},
		{name: "integer rejects a fraction", args: `{"sku":"A-1","quantity":2.5}`, wantPath: "$.quantity", wantError: true},
		{name: "enum rejects another value", args: `{"sku":"A-1","quantity":1,"priority":"MEDIUM"}`, wantPath: "$.priority", wantError: true},
		{name: "unknown property rejected by additionalProperties false", args: `{"sku":"A-1","quantity":1,"color":"red"}`, wantPath: "$", wantError: true},
		{name: "nested item missing its required id", args: `{"sku":"A-1","quantity":1,"items":[{"id":"x"},{}]}`, wantPath: "$.items[1]", wantError: true},
		{name: "top-level array where an object is required", args: `[1]`, wantPath: "$", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateArguments(schema, mustParse(t, tt.args))
			if !tt.wantError {
				if err != nil {
					t.Fatalf("ValidateArguments(%s) = %v, want nil", tt.args, err)
				}
				return
			}
			var ae *ArgumentError
			if !errors.As(err, &ae) {
				t.Fatalf("ValidateArguments(%s) error = %v, want *ArgumentError", tt.args, err)
			}
			if ae.Path != tt.wantPath {
				t.Errorf("ValidateArguments(%s) path = %q, want %q", tt.args, ae.Path, tt.wantPath)
			}
		})
	}
}

func TestArgumentValidationZeroSchemaAcceptsAll(t *testing.T) {
	t.Parallel()
	if err := ValidateArguments(jsonvalue.Value{}, mustParse(t, `{"anything":true}`)); err != nil {
		t.Errorf("ValidateArguments with a zero schema = %v, want nil", err)
	}
}
