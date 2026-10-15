// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarativejson

import (
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/connector/declarative"
)

const sample = `{
  "format": "orcon/declarative@1",
  "http": {"baseUrl": "https://api.example.com", "auth": {"type": "BEARER"}},
  "operations": {
    "item.list": {
      "purity": "READ",
      "safety": "READ_ONLY",
      "request": {"method": "GET", "path": "/items"},
      "response": {"select": "$.data"},
      "pagination": {"strategy": "OFFSET", "param": "offset", "limitParam": "limit", "pageSize": 50}
    }
  }
}`

func TestDecodeRoundTripsAValidDocument(t *testing.T) {
	def, err := Decode([]byte(sample))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if err := declarative.Validate(def); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	op := def.Operations["item.list"]
	if op.Pagination == nil || op.Pagination.PageSize != 50 {
		t.Fatalf("pagination not decoded: %+v", op.Pagination)
	}
}

func TestDecodeRefusesUnknownMembers(t *testing.T) {
	bad := strings.Replace(sample, `"purity": "READ"`, `"purity": "READ", "purty": "x"`, 1)
	if _, err := Decode([]byte(bad)); err == nil {
		t.Fatal("Decode accepted an unknown member")
	}
}

func TestDecodeRefusesTrailingData(t *testing.T) {
	if _, err := Decode([]byte(sample + ` {}`)); err == nil {
		t.Fatal("Decode accepted trailing data")
	}
}
