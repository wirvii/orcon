// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestClassifyResponseByStatusAndSafety(t *testing.T) {
	ruleOn := func(op Operation, rule ErrorRule) Operation {
		op.Response.Errors = []ErrorRule{rule}
		return op
	}
	cases := []struct {
		name      string
		op        Operation
		resp      HTTPResponse
		class     Class
		code      string
		action    Action
		ambiguous bool
		retry     time.Duration
	}{
		{"READ 404 is a definite refusal", readOp("/items"), HTTPResponse{Status: 404}, ClassPermanent, "NotFound", ActionNone, false, 0},
		{"UNSAFE 400 is a definite refusal, not an unknown outcome", writeOp(SafetyUnsafe), HTTPResponse{Status: 400}, ClassPermanent, "ValidationFailed", ActionNone, false, 0},
		{"IDEMPOTENCY_KEY 409 is a permanent conflict", writeOp(SafetyIdempotencyKey), HTTPResponse{Status: 409}, ClassPermanent, "Conflict", ActionNone, false, 0},
		{"READ 402 maps to QuotaExceeded", readOp("/items"), HTTPResponse{Status: 402}, ClassPermanent, "QuotaExceeded", ActionNone, false, 0},
		{"READ 418 maps to ProviderError", readOp("/items"), HTTPResponse{Status: 418}, ClassPermanent, "ProviderError", ActionNone, false, 0},
		{"READ 504 is a transient timeout", readOp("/items"), HTTPResponse{Status: 504}, ClassTransient, "Timeout", ActionNone, false, 0},
		{"UNSAFE 429 is not applied and carries Retry-After", writeOp(SafetyUnsafe),
			HTTPResponse{Status: 429, Header: map[string][]string{"Retry-After": {"7"}}}, ClassTransient, "RateLimited", ActionNone, false, 7 * time.Second},
		{"UNSAFE 500 is UNCERTAIN", writeOp(SafetyUnsafe), HTTPResponse{Status: 500}, ClassUncertain, "ProviderError", ActionSuspend, true, 0},
		{"RECONCILE 500 asks for verification", writeOp(SafetyReconcile), HTTPResponse{Status: 500}, ClassTransient, "ProviderError", ActionVerify, true, 0},
		{"explicit rule keeps its own code", ruleOn(readOp("/items"), ErrorRule{Status: []int{409}, Class: "PERMANENT", Code: "QuotaExceeded"}),
			HTTPResponse{Status: 409}, ClassPermanent, "QuotaExceeded", ActionNone, false, 0},
		{"explicit rule without code uses the status default", ruleOn(readOp("/items"), ErrorRule{Status: []int{404}, Class: "PERMANENT"}),
			HTTPResponse{Status: 404}, ClassPermanent, "NotFound", ActionNone, false, 0},
		{"explicit rule can mark a READ failure transient", ruleOn(readOp("/items"), ErrorRule{Status: []int{403}, Class: "TRANSIENT", Code: "PermissionDenied"}),
			HTTPResponse{Status: 403}, ClassTransient, "PermissionDenied", ActionNone, false, 0},
		{"a rule on UNSAFE 503 cannot make an unknown outcome definite", ruleOn(writeOp(SafetyUnsafe), ErrorRule{Status: []int{503}, Class: "PERMANENT", Code: "ServiceUnavailable"}),
			HTTPResponse{Status: 503}, ClassUncertain, "ServiceUnavailable", ActionSuspend, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := classifyResponse(tc.op, tc.resp)
			if f == nil {
				t.Fatal("classifyResponse returned nil for a failure")
			}
			if f.Class != tc.class || f.Code != tc.code || f.Action != tc.action || f.Ambiguous != tc.ambiguous || f.RetryAfter != tc.retry {
				t.Fatalf("class=%s code=%s action=%s ambiguous=%v retry=%v", f.Class, f.Code, f.Action, f.Ambiguous, f.RetryAfter)
			}
		})
	}
}

func TestClassifyResponseAcceptsEverySuccessStatus(t *testing.T) {
	for _, status := range []int{200, 201, 204, 299} {
		if f := classifyResponse(readOp("/items"), HTTPResponse{Status: status}); f != nil {
			t.Errorf("status %d classified as %s", status, f.Class)
		}
	}
}

func TestDefaultCodeStaysInsideTheClosedList(t *testing.T) {
	want := map[int]string{
		400: "ValidationFailed", 401: "AuthInvalid", 402: "QuotaExceeded", 403: "PermissionDenied",
		404: "NotFound", 408: "Timeout", 409: "Conflict", 413: "PayloadTooLarge", 422: "ValidationFailed",
		429: "RateLimited", 503: "ServiceUnavailable", 504: "Timeout", 418: "ProviderError", 500: "ProviderError",
	}
	for status, code := range want {
		if got := defaultCode(status); got != code {
			t.Errorf("defaultCode(%d) = %s, want %s", status, got, code)
		}
		if !IsCode(code) {
			t.Errorf("%s is not in the closed list", code)
		}
	}
}

func TestTransportErrorsOnReadAreNeverAmbiguous(t *testing.T) {
	f := classifyTransport(readOp("/items"), context.DeadlineExceeded)
	if f.Ambiguous || f.Action != ActionNone || f.Class != ClassTransient || f.Code != "Timeout" {
		t.Fatalf("class=%s code=%s ambiguous=%v action=%s", f.Class, f.Code, f.Ambiguous, f.Action)
	}
}

func TestCanonicalQueryIsSortedAndAWSEscaped(t *testing.T) {
	q := url.Values{"b": {"2"}, "a": {"y", "x z"}}
	if got := canonicalQuery(q); got != "a=x%20z&a=y&b=2" {
		t.Fatalf("canonicalQuery = %q", got)
	}
	if got := canonicalQuery(url.Values{}); got != "" {
		t.Fatalf("empty query = %q", got)
	}
	if got := awsEscape("a/b c~"); got != "a%2Fb%20c~" {
		t.Fatalf("awsEscape = %q", got)
	}
}

func TestAPIKeyFieldSelectsTheCredential(t *testing.T) {
	s := okSender(`{"data":[]}`)
	auth := Auth{Type: AuthAPIKey, In: "HEADER", Name: "X-Key", Field: "token", Prefix: "k_"}
	in := newInterp(t, baseDef(auth, map[string]Operation{"item.list": readOp("/items")}), s, nil)
	if _, err := in.Execute(t.Context(), Call{Operation: "item.list", Credentials: map[string]string{"token": "abc"}}, nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := s.calls[0].Header["X-Key"]; len(got) != 1 || got[0] != "k_abc" {
		t.Fatalf("X-Key = %v", got)
	}
}

func TestCookieAuthKeepsTheOperationsOwnCookies(t *testing.T) {
	op := readOp("/items")
	op.Request.Headers = map[string]string{"Cookie": "theme=dark"}
	s := okSender(`{"data":[]}`)
	in := newInterp(t, baseDef(Auth{Type: AuthAPIKey, In: "COOKIE", Name: "sid"}, map[string]Operation{"item.list": op}), s, nil)
	if _, err := in.Execute(t.Context(), Call{Operation: "item.list", Credentials: map[string]string{"apiKey": "KEY1"}}, nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := []string{"theme=dark", "sid=KEY1"}
	if got := s.calls[0].Header["Cookie"]; !slices.Equal(got, want) {
		t.Fatalf("Cookie = %v, want %v", got, want)
	}
}

func TestSigV4SignsTheSessionTokenAndTheQuery(t *testing.T) {
	auth := Auth{Type: AuthAWSSigV4, Service: "s3", Region: "us-east-1"}
	sign := func(query string, creds map[string]string) HTTPRequest {
		op := readOp("/items")
		op.Request.Query = map[string]string{"q": query}
		s := okSender(`{"data":[]}`)
		in := newInterp(t, baseDef(auth, map[string]Operation{"item.list": op}), s, nil)
		if _, err := in.Execute(t.Context(), Call{Operation: "item.list", Credentials: creds}, nil); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		return s.calls[0]
	}
	base := map[string]string{"accessKeyId": "AKID", "secretAccessKey": "SECRET"}
	withToken := map[string]string{"accessKeyId": "AKID", "secretAccessKey": "SECRET", "sessionToken": "SESS"}

	req := sign("one", withToken)
	if got := req.Header["X-Amz-Security-Token"]; len(got) != 1 || got[0] != "SESS" {
		t.Fatalf("X-Amz-Security-Token = %v", got)
	}
	if header := req.Header["Authorization"][0]; !strings.Contains(header, "SignedHeaders=host;x-amz-date;x-amz-security-token,") {
		t.Fatalf("the session token is not signed: %s", header)
	}
	if sign("one", base).Header["Authorization"][0] == sign("two", base).Header["Authorization"][0] {
		t.Fatal("the signature does not cover the query")
	}
}

func TestSigV4SignatureCoversTheBody(t *testing.T) {
	auth := Auth{Type: AuthAWSSigV4, Service: "s3", Region: "us-east-1"}
	sign := func(value string) string {
		op := Operation{
			Purity: PurityEffect, Safety: SafetyUnsafe,
			Request: Request{Method: "PUT", Path: "/objects", Body: &Body{Type: "JSON", Fields: map[string]string{"v": "{{$parameter.v}}"}}},
		}
		s := okSender(`{}`)
		in := newInterp(t, baseDef(auth, map[string]Operation{"object.put": op}), s, nil)
		params := map[string]jsonvalue.Value{"v": jsonvalue.String(value)}
		creds := map[string]string{"accessKeyId": "AKID", "secretAccessKey": "SECRET"}
		if _, err := in.Execute(t.Context(), Call{Operation: "object.put", Parameters: params, Credentials: creds}, nil); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		return s.calls[0].Header["Authorization"][0]
	}
	if sign("a") == sign("b") {
		t.Fatal("the signature does not cover the body")
	}
}
