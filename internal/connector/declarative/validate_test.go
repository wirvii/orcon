// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// opDef wraps one operation named item.list in a valid connector.
func opDef(op Operation) Definition {
	return baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op})
}

func TestValidateAcceptsWellFormedDefinition(t *testing.T) {
	head := readOp("/items")
	head.Request.Method = "HEAD"
	paged := readOp("/items")
	paged.Pagination = &Pagination{Strategy: "OFFSET", Param: "offset", LimitParam: "limit", PageSize: 50, MaxPages: 10, OnMaxPages: "TRUNCATE"}
	effect := writeOp(SafetyUnsafe)
	effect.Response.Errors = []ErrorRule{{Status: []int{500}, Class: "UNCERTAIN", Code: "ProviderError"}}
	def := baseDef(Auth{Type: AuthAPIKey, In: "COOKIE", Name: "sid"}, map[string]Operation{
		"item.head": head, "item.list": paged, "thing.create": effect,
	})
	def.HTTP.Timeout = "30s"
	if err := Validate(def); err != nil {
		t.Fatalf("a well-formed definition was refused: %v", err)
	}
}

func TestValidateRefusesBrokenDefinitions(t *testing.T) {
	cases := []struct {
		name string
		def  func() Definition
		want string
	}{
		{"wrong format", func() Definition { d := opDef(readOp("/items")); d.Format = "x"; return d }, "format"},
		{"relative base URL", func() Definition { d := opDef(readOp("/items")); d.HTTP.BaseURL = "/v1"; return d }, "http/baseUrl"},
		{"non-HTTP scheme", func() Definition { d := opDef(readOp("/items")); d.HTTP.BaseURL = "ftp://x"; return d }, "http/baseUrl"},
		{"unparseable timeout", func() Definition { d := opDef(readOp("/items")); d.HTTP.Timeout = "soon"; return d }, "http/timeout"},
		{"zero timeout", func() Definition { d := opDef(readOp("/items")); d.HTTP.Timeout = "0s"; return d }, "http/timeout"},
		{"no operations", func() Definition { return baseDef(Auth{Type: AuthNone}, nil) }, "operations"},
		{"name without a verb", func() Definition {
			return baseDef(Auth{Type: AuthNone}, map[string]Operation{"item": readOp("/items")})
		}, "operations/item"},
		{"name with three parts", func() Definition {
			return baseDef(Auth{Type: AuthNone}, map[string]Operation{"a.b.c": readOp("/items")})
		}, "operations/a.b.c"},
		{"unknown purity", readWith(func(o *Operation) { o.Purity = "MAYBE" }), "/purity"},
		{"unknown safety", readWith(func(o *Operation) { o.Safety = "SOMETIMES" }), "/safety"},
		{"READ that is not READ_ONLY", readWith(func(o *Operation) { o.Safety = SafetyUnsafe }), "a READ operation is READ_ONLY"},
		{"EFFECT that is READ_ONLY", readWith(func(o *Operation) { o.Purity = PurityEffect }), "an EFFECT operation cannot be READ_ONLY"},
		{"IDEMPOTENCY_KEY without header", func() Definition {
			o := writeOp(SafetyIdempotencyKey)
			o.IdempotencyHeader = ""
			return opDef(o)
		}, "/idempotencyKey"},
		{"method the interpreter does not send", readWith(func(o *Operation) { o.Request.Method = "TRACE" }), "/method"},
		{"READ with POST", func() Definition {
			o := readOp("/items")
			o.Request.Method = "POST"
			return opDef(o)
		}, "a READ operation uses GET or HEAD"},
		{"path without a leading slash", readWith(func(o *Operation) { o.Request.Path = "items" }), "/path"},
		{"unknown body type", func() Definition {
			o := writeOp(SafetyUnsafe)
			o.Request.Body = &Body{Type: "XML"}
			return opDef(o)
		}, "/body/type"},
		{"select without $", readWith(func(o *Operation) { o.Response.Select = "data" }), "/select"},
		{"select with a filter", readWith(func(o *Operation) { o.Response.Select = "$.data[*]" }), "/select"},
		{"unknown empty mode", readWith(func(o *Operation) { o.Response.Empty = "MANY" }), "/empty"},
		{"error rule with an unknown class", readWith(func(o *Operation) {
			o.Response.Errors = []ErrorRule{{Status: []int{404}, Class: "FATAL"}}
		}), "/class"},
		{"error rule with an invented code", readWith(func(o *Operation) {
			o.Response.Errors = []ErrorRule{{Status: []int{404}, Class: "PERMANENT", Code: "Teapot"}}
		}), "/code"},
		{"error rule without a status", readWith(func(o *Operation) {
			o.Response.Errors = []ErrorRule{{Class: "PERMANENT"}}
		}), "needs a status code"},
		{"error rule with an impossible status", readWith(func(o *Operation) {
			o.Response.Errors = []ErrorRule{{Status: []int{42}, Class: "PERMANENT"}}
		}), "/status"},
		{"unknown pagination strategy", readWith(func(o *Operation) { o.Pagination = &Pagination{Strategy: "SEEK"} }), "/strategy"},
		{"OFFSET without its parameter", readWith(func(o *Operation) { o.Pagination = &Pagination{Strategy: "OFFSET"} }), "/param"},
		{"cursor without a source", readWith(func(o *Operation) { o.Pagination = &Pagination{Strategy: "CURSOR", Param: "c", From: "BODY"} }), "CURSOR needs param and from PATH or HEADER"},
		{"cursor from path without a path", readWith(func(o *Operation) { o.Pagination = &Pagination{Strategy: "CURSOR", Param: "c", From: "PATH"} }), "CURSOR needs the path or header it reads"},
		{"cursor from header without a name", readWith(func(o *Operation) { o.Pagination = &Pagination{Strategy: "CURSOR", Param: "c", From: "HEADER"} }), "CURSOR needs the path or header it reads"},
		{"NEXT_URL without a path", readWith(func(o *Operation) { o.Pagination = &Pagination{Strategy: "NEXT_URL"} }), "/path"},
		{"pagination path with an index that is not a number", readWith(func(o *Operation) {
			o.Pagination = &Pagination{Strategy: "NEXT_URL", Path: "$.next[x]"}
		}), "/path"},
		{"negative maxPages", readWith(func(o *Operation) { o.Pagination = &Pagination{Strategy: "PAGE", Param: "p", MaxPages: -1} }), "/maxPages"},
		{"unknown onMaxPages", readWith(func(o *Operation) { o.Pagination = &Pagination{Strategy: "PAGE", Param: "p", OnMaxPages: "STOP"} }), "/onMaxPages"},
		{"negative maxAttempts", readWith(func(o *Operation) { o.MaxAttempts = -1 }), "/maxAttempts"},
		{"credential in a query value", readWith(func(o *Operation) {
			o.Request.Query = map[string]string{"k": "{{$credentials.token}}"}
		}), "$credentials is visible only to the authentication step"},
		{"credential in a header", readWith(func(o *Operation) {
			o.Request.Headers = map[string]string{"X-T": "{{$credentials.token}}"}
		}), "$credentials is visible only to the authentication step"},
		{"credential in a body field", func() Definition {
			o := writeOp(SafetyUnsafe)
			o.Request.Body = &Body{Type: "JSON", Fields: map[string]Node{"t": {Value: jsonvalue.String("{{$credentials.token}}")}}}
			return opDef(o)
		}, "$credentials is visible only to the authentication step"},
		{"credential in the idempotency key", func() Definition {
			o := writeOp(SafetyIdempotencyKey)
			o.IdempotencyKey = "{{$credentials.token}}"
			return opDef(o)
		}, "$credentials is visible only to the authentication step"},
		{"unknown authentication placement", func() Definition {
			return baseDef(Auth{Type: AuthAPIKey, In: "BODY", Name: "k"}, map[string]Operation{"item.list": readOp("/items")})
		}, "http/auth/in"},
		{"API_KEY without a name", func() Definition {
			return baseDef(Auth{Type: AuthAPIKey, In: "HEADER"}, map[string]Operation{"item.list": readOp("/items")})
		}, "http/auth/name"},
		{"CUSTOM_HEADERS without headers", func() Definition {
			return baseDef(Auth{Type: AuthCustomHeaders}, map[string]Operation{"item.list": readOp("/items")})
		}, "http/auth/headers"},
		{"AWS_SIG_V4 without a region", func() Definition {
			return baseDef(Auth{Type: AuthAWSSigV4, Service: "s3"}, map[string]Operation{"item.list": readOp("/items")})
		}, "http/auth"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.def())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate = %v, want a problem mentioning %q", err, tc.want)
			}
		})
	}
}

func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	d := baseDef(Auth{Type: AuthNone}, nil)
	d.Format = "x"
	err := Validate(d)
	if err == nil || !strings.Contains(err.Error(), "format") || !strings.Contains(err.Error(), "operations") {
		t.Fatalf("Validate = %v, want both problems", err)
	}
}

// readWith builds a valid READ definition and applies change to its operation.
func readWith(change func(*Operation)) func() Definition {
	return func() Definition {
		o := readOp("/items")
		change(&o)
		return opDef(o)
	}
}
