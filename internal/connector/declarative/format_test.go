// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"testing"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// runAgainst executes item.list of def once against a fixed reply.
func runAgainst(t *testing.T, def Definition, status int, body string) (Result, error) {
	t.Helper()
	s := &fakeSender{respond: func(int, HTTPRequest) (HTTPResponse, error) { return reply(status, body), nil }}
	in := newInterp(t, def, s, nil)
	return in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
}

// TestBodyCodeRuleClassifiesThrottling: a provider that answers a throughput
// limit with HTTP 400 is transient because of the code in the body, while the
// same status with another code stays a validation failure.
func TestBodyCodeRuleClassifiesThrottling(t *testing.T) {
	op := readOp("/items")
	op.Response.Errors = []ErrorRule{
		{Body: &BodyMatch{Path: "$.error.code", In: []Node{{Value: jsonvalue.Int(4)}, {Value: jsonvalue.String("80007")}}}, Class: "TRANSIENT", Code: "RateLimited"},
		{Status: []int{400}, Class: "PERMANENT", Code: "ValidationFailed"},
	}
	def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op})

	cases := []struct {
		name, body, class, code string
	}{
		{"numeric code", `{"error":{"message":"Too many calls","code":4}}`, "TRANSIENT", "RateLimited"},
		{"code given as text", `{"error":{"code":"80007"}}`, "TRANSIENT", "RateLimited"},
		{"other code on the same status", `{"error":{"message":"(#100) Invalid parameter","code":100}}`, "PERMANENT", "ValidationFailed"},
		{"body is not JSON", `<html>400</html>`, "PERMANENT", "ValidationFailed"},
		{"code member missing", `{"error":{"message":"bad"}}`, "PERMANENT", "ValidationFailed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := runAgainst(t, def, 400, c.body)
			f := asFailure(t, err)
			if string(f.Class) != c.class || f.Code != c.code {
				t.Fatalf("failure is %s/%s, want %s/%s", f.Class, f.Code, c.class, c.code)
			}
		})
	}
}

// TestBodyRuleWithoutStatusMatchesAnyFailure: a body rule that names no status
// applies to whatever status carries the code.
func TestBodyRuleWithoutStatusMatchesAnyFailure(t *testing.T) {
	op := readOp("/items")
	op.Response.Errors = []ErrorRule{
		{Body: &BodyMatch{Path: "$.error.code", In: []Node{{Value: jsonvalue.Int(613)}}}, Class: "TRANSIENT", Code: "RateLimited"},
	}
	def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op})
	_, err := runAgainst(t, def, 422, `{"error":{"code":613}}`)
	if f := asFailure(t, err); f.Class != ClassTransient || f.Code != "RateLimited" {
		t.Fatalf("failure is %s/%s, want TRANSIENT/RateLimited", f.Class, f.Code)
	}
}

// TestRuleOrderDecidesTheFirstMatch: rules are tried in order, so a body rule
// placed after the status rule of the same response never takes effect.
func TestRuleOrderDecidesTheFirstMatch(t *testing.T) {
	op := readOp("/items")
	op.Response.Errors = []ErrorRule{
		{Status: []int{400}, Class: "PERMANENT", Code: "ValidationFailed"},
		{Body: &BodyMatch{Path: "$.error.code", In: []Node{{Value: jsonvalue.Int(4)}}}, Class: "TRANSIENT", Code: "RateLimited"},
	}
	def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op})
	_, err := runAgainst(t, def, 400, `{"error":{"code":4}}`)
	if f := asFailure(t, err); f.Class != ClassPermanent {
		t.Fatalf("failure is %s, want PERMANENT from the first rule", f.Class)
	}
}

// TestUncertainWriteIgnoresBodyRules: an unknown outcome of an UNSAFE write
// stays UNCERTAIN even when a body rule would call it transient.
func TestUncertainWriteIgnoresBodyRules(t *testing.T) {
	op := writeOp(SafetyUnsafe)
	op.Response.Errors = []ErrorRule{
		{Body: &BodyMatch{Path: "$.error.code", In: []Node{{Value: jsonvalue.Int(4)}}}, Class: "TRANSIENT", Code: "RateLimited"},
	}
	def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"thing.create": op})
	s := &fakeSender{respond: func(int, HTTPRequest) (HTTPResponse, error) {
		return reply(500, `{"error":{"code":4}}`), nil
	}}
	in := newInterp(t, def, s, nil)
	_, err := in.Execute(t.Context(), Call{Operation: "thing.create"}, nil)
	if f := asFailure(t, err); f.Class != ClassUncertain || len(s.calls) != 1 {
		t.Fatalf("failure is %s after %d sends, want UNCERTAIN after one send", f.Class, len(s.calls))
	}
}

// TestResponseMapAddsKeysAndKeepsTheRecord: the mapped keys are added to each
// emitted item and the provider's record stays as it is; a path that finds
// nothing adds nothing.
func TestResponseMapAddsKeysAndKeepsTheRecord(t *testing.T) {
	op := readOp("/items")
	op.Response = ResponseSpec{
		Select: "$.data",
		Map:    map[string]string{"id": "$.messages[0].id", "recipient": "$.contacts[0].wa_id", "absent": "$.nope"},
	}
	def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op})
	body := `{"data":{"messaging_product":"whatsapp","contacts":[{"input":"+57","wa_id":"573000000000"}],"messages":[{"id":"wamid.1"}]}}`
	res, err := runAgainst(t, def, 200, body)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(res.Items))
	}
	got, err := jsonvalue.Canonical(res.Items[0])
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	const want = `{"contacts":[{"input":"+57","wa_id":"573000000000"}],"id":"wamid.1","messages":[{"id":"wamid.1"}],"messaging_product":"whatsapp","recipient":"573000000000"}`
	if string(got) != want {
		t.Fatalf("item is %s, want %s", got, want)
	}
}

// TestNestedBodyRendersAtAnyDepth: nested objects and arrays keep their JSON
// types, a placeholder alone keeps the parameter's type, and text placeholders
// are filled inside the nesting.
func TestNestedBodyRendersAtAnyDepth(t *testing.T) {
	text, err := jsonvalue.Object(
		jsonvalue.Member{Key: "body", Value: jsonvalue.String("hola {{$parameter.name}}")},
		jsonvalue.Member{Key: "preview_url", Value: jsonvalue.Bool(false)},
	)
	if err != nil {
		t.Fatal(err)
	}
	buttons := jsonvalue.Array(jsonvalue.String("{{$parameter.first}}"), jsonvalue.Int(2))
	op := writeOp(SafetyUnsafe)
	op.Request.Body = &Body{Type: "JSON", Fields: map[string]Node{
		"text":    {Value: text},
		"buttons": {Value: buttons},
		"payload": {Value: jsonvalue.String("{{$parameter.payload}}")},
	}}
	def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"thing.create": op})
	s := okSender(`{}`)
	in := newInterp(t, def, s, nil)
	_, err = in.Execute(t.Context(), Call{Operation: "thing.create", Parameters: map[string]jsonvalue.Value{
		"name":    jsonvalue.String("Ana"),
		"first":   jsonvalue.String("si"),
		"payload": mustObject(t, jsonvalue.Member{Key: "id", Value: jsonvalue.Int(7)}),
	}}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got, err := jsonvalue.Canonical(mustParse(t, s.calls[0].Body))
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	const want = `{"buttons":["si",2],"payload":{"id":7},"text":{"body":"hola Ana","preview_url":false}}`
	if string(got) != want {
		t.Fatalf("body is %s, want %s", got, want)
	}
}

// TestBodyRulesValidate: the shape checks that keep a nested body and a body
// condition from running in an unsafe or meaningless way.
func TestBodyRulesValidate(t *testing.T) {
	obj := mustObject(t, jsonvalue.Member{Key: "a", Value: jsonvalue.Int(1)})
	cases := map[string]func(*Definition){
		"rule with neither status nor body": func(d *Definition) {
			op := d.Operations["item.list"]
			op.Response.Errors = []ErrorRule{{Class: "PERMANENT"}}
			d.Operations["item.list"] = op
		},
		"body condition on an object": func(d *Definition) {
			op := d.Operations["item.list"]
			op.Response.Errors = []ErrorRule{{Body: &BodyMatch{Path: "$.error", In: []Node{{Value: obj}}}, Class: "PERMANENT"}}
			d.Operations["item.list"] = op
		},
		"body condition with an invalid path": func(d *Definition) {
			op := d.Operations["item.list"]
			op.Response.Errors = []ErrorRule{{Body: &BodyMatch{Path: "error.code", In: []Node{{Value: jsonvalue.Int(4)}}}, Class: "PERMANENT"}}
			d.Operations["item.list"] = op
		},
		"response map with a path that is not absolute": func(d *Definition) {
			op := d.Operations["item.list"]
			op.Response.Map = map[string]string{"id": "messages.id"}
			d.Operations["item.list"] = op
		},
		"credential inside a nested body": func(d *Definition) {
			op := writeOp(SafetyUnsafe)
			inner := mustObject(t, jsonvalue.Member{Key: "k", Value: jsonvalue.String("{{$credentials.token}}")})
			op.Request.Body = &Body{Type: "JSON", Fields: map[string]Node{"x": {Value: inner}}}
			d.Operations["thing.create"] = op
		},
		"credential inside a RAW body": func(d *Definition) {
			op := writeOp(SafetyUnsafe)
			op.Request.Body = &Body{Type: "RAW", Raw: "token={{$credentials.token}}"}
			d.Operations["thing.create"] = op
		},
		"form field that is not a string": func(d *Definition) {
			op := writeOp(SafetyUnsafe)
			op.Request.Body = &Body{Type: "FORM", Fields: map[string]Node{"x": {Value: jsonvalue.Int(1)}}}
			d.Operations["thing.create"] = op
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": readOp("/items")})
			mutate(&def)
			if err := Validate(def); err == nil {
				t.Fatal("Validate accepted the definition")
			}
		})
	}
}

func mustObject(t *testing.T, members ...jsonvalue.Member) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Object(members...)
	if err != nil {
		t.Fatalf("Object: %v", err)
	}
	return v
}

func mustParse(t *testing.T, data []byte) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return v
}
