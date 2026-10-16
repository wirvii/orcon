// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// The request builders and the classification helpers are exercised here directly:
// their branches are the rules of the connector's wire format, and an interpreter
// test that reaches them only through one operation leaves most of them untried.

func builderScope() scope {
	return scope{
		params: map[string]jsonvalue.Value{
			"name": jsonvalue.String("bob"),
			"n":    jsonvalue.Int(3),
		},
		credentials: map[string]string{"token": "secret"},
	}
}

func TestRawFormAndJSONBodies(t *testing.T) {
	in := &Interpreter{def: Definition{HTTP: HTTP{Headers: map[string]string{"X-Name": "{{$parameter.name}}"}}}}
	params := builderScope()

	raw := Operation{Request: Request{Body: &Body{Type: "RAW", Raw: "hi {{$parameter.name}}"}}}
	req := HTTPRequest{}
	if err := in.addBody(raw, params, &req); err != nil || string(req.Body) != "hi bob" {
		t.Fatalf("RAW body = %q, %v", req.Body, err)
	}

	form := Operation{Request: Request{Body: &Body{Type: "FORM", Fields: map[string]Node{"who": {Value: jsonvalue.String("{{$parameter.name}}")}}}}}
	req = HTTPRequest{Header: map[string][]string{}}
	if err := in.addBody(form, params, &req); err != nil || string(req.Body) != "who=bob" {
		t.Fatalf("FORM body = %q, %v", req.Body, err)
	}
	if got := req.Header["Content-Type"]; len(got) != 1 || got[0] != "application/x-www-form-urlencoded" {
		t.Fatalf("FORM content type = %v", got)
	}

	obj := Operation{Request: Request{Body: &Body{Type: "JSON", Fields: map[string]Node{
		"count": {Value: jsonvalue.String("{{$parameter.n}}")},
		"label": {Value: jsonvalue.String("x {{$parameter.name}}")},
	}}}}
	req = HTTPRequest{Header: map[string][]string{}}
	if err := in.addBody(obj, params, &req); err != nil {
		t.Fatalf("JSON body: %v", err)
	}
	if string(req.Body) != `{"count":3,"label":"x bob"}` {
		t.Fatalf("JSON body = %s", req.Body)
	}
	if got := req.Header["Content-Type"]; len(got) != 1 || got[0] != "application/json" {
		t.Fatalf("JSON content type = %v", got)
	}

	missing := Operation{Request: Request{Body: &Body{Type: "RAW", Raw: "{{$parameter.absent}}"}}}
	if err := in.addBody(missing, params, &HTTPRequest{}); !errors.Is(err, ErrTemplate) {
		t.Fatalf("a missing parameter in a RAW body: %v", err)
	}
	none := Operation{}
	if err := in.addBody(none, params, &HTTPRequest{}); err != nil {
		t.Fatalf("no body: %v", err)
	}
}

func TestHeadersRenderAndFailOnMissingParameters(t *testing.T) {
	in := &Interpreter{def: Definition{HTTP: HTTP{Headers: map[string]string{"X-Name": "{{$parameter.name}}"}}}}
	params := builderScope()
	req := HTTPRequest{Header: map[string][]string{}}
	op := Operation{Request: Request{Headers: map[string]string{"X-Trace": "tok {{$parameter.name}}"}}}
	if err := in.addHeaders(op, params, &req); err != nil {
		t.Fatalf("addHeaders: %v", err)
	}
	if req.Header["X-Name"][0] != "bob" || req.Header["X-Trace"][0] != "tok bob" {
		t.Fatalf("headers = %v", req.Header)
	}

	bad := Operation{Request: Request{Headers: map[string]string{"X-Bad": "{{$parameter.absent}}"}}}
	if err := in.addHeaders(bad, params, &HTTPRequest{Header: map[string][]string{}}); !errors.Is(err, ErrTemplate) {
		t.Fatalf("a missing parameter in a header: %v", err)
	}
	badDef := &Interpreter{def: Definition{HTTP: HTTP{Headers: map[string]string{"X-Bad": "{{$parameter.absent}}"}}}}
	if err := badDef.addHeaders(Operation{}, params, &HTTPRequest{Header: map[string][]string{}}); !errors.Is(err, ErrTemplate) {
		t.Fatalf("a missing parameter in a definition header: %v", err)
	}
}

func TestSinglePlaceholderAndRenderValue(t *testing.T) {
	if p, ok := singlePlaceholder("{{$parameter.a}}"); !ok || p != "$parameter.a" {
		t.Fatalf("single placeholder = %q, %v", p, ok)
	}
	for _, tpl := range []string{"x {{a}}", "{{a}}{{b}}", "plain"} {
		if _, ok := singlePlaceholder(tpl); ok {
			t.Fatalf("%q is not a single placeholder", tpl)
		}
	}

	params := builderScope()
	v, err := params.renderValue("{{$parameter.n}}")
	if err != nil || v.Kind() != jsonvalue.KindNumber {
		t.Fatalf("a single parameter keeps its JSON type: %v, %v", v.Kind(), err)
	}
	if _, err := params.renderValue("{{$parameter.absent}}"); !errors.Is(err, ErrTemplate) {
		t.Fatalf("a missing single parameter: %v", err)
	}
	text, err := params.renderValue("hello {{$parameter.name}}")
	if err != nil {
		t.Fatalf("text: %v", err)
	}
	if s, ok := text.Str(); !ok || s != "hello bob" {
		t.Fatalf("text value = %v", text)
	}
}

func TestScalarTextRefusesStructures(t *testing.T) {
	cases := map[string]string{
		`"abc"`: "abc",
		`42`:    "42",
		`true`:  "true",
		`false`: "false",
	}
	for in, want := range cases {
		v, err := jsonvalue.Parse([]byte(in))
		if err != nil {
			t.Fatalf("parse %s: %v", in, err)
		}
		got, err := scalarText(v)
		if err != nil || got != want {
			t.Fatalf("scalarText(%s) = %q, %v; want %q", in, got, err, want)
		}
	}
	obj, _ := jsonvalue.Parse([]byte(`{"a":1}`))
	if _, err := scalarText(obj); err == nil {
		t.Fatal("an object was serialized as text")
	}
}

func TestEscapingAndCanonicalQuery(t *testing.T) {
	if got := awsEscape("a b+c/"); got != "a%20b%2Bc%2F" {
		t.Fatalf("awsEscape = %q", got)
	}
	q := url.Values{"b": {"2", "1"}, "a": {"x y"}}
	if got := canonicalQuery(q); got != "a=x%20y&b=1&b=2" {
		t.Fatalf("canonicalQuery = %q", got)
	}
}

func TestStatusClassificationHelpers(t *testing.T) {
	for _, status := range []int{400, 401, 402, 403, 404, 408, 409, 413, 422, 429, 503, 504, 418} {
		if defaultCode(status) == "" {
			t.Fatalf("no code for status %d", status)
		}
	}
	if defaultCode(402) != "QuotaExceeded" || defaultCode(418) != "ProviderError" {
		t.Fatal("unexpected default codes")
	}
	if !containsStatus([]int{200, 503}, 503) || containsStatus([]int{200}, 500) {
		t.Fatal("containsStatus")
	}
	for header, want := range map[string]time.Duration{"7": 7 * time.Second, "abc": 0, "-3": 0} {
		resp := HTTPResponse{Header: map[string][]string{"Retry-After": {header}}}
		if got := retryAfterOf(resp); got != want {
			t.Fatalf("Retry-After %q = %v, want %v", header, got, want)
		}
	}
	if got := retryAfterOf(HTTPResponse{}); got != 0 {
		t.Fatalf("no Retry-After = %v", got)
	}
}

func TestRequestFailureCodes(t *testing.T) {
	if got := requestFailure(ErrMissingField); got.Code != "AuthInvalid" {
		t.Fatalf("missing field code = %s", got.Code)
	}
	if got := requestFailure(ErrUnsupported); got.Code != "Unsupported" {
		t.Fatalf("unsupported code = %s", got.Code)
	}
	if got := requestFailure(errors.New("other")); got.Code != "ValidationFailed" {
		t.Fatalf("other code = %s", got.Code)
	}
}
