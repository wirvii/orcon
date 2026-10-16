// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// fakeSender answers from a script and records every request it receives.
type fakeSender struct {
	calls   []HTTPRequest
	respond func(n int, req HTTPRequest) (HTTPResponse, error)
}

func (f *fakeSender) Send(_ context.Context, req HTTPRequest) (HTTPResponse, error) {
	f.calls = append(f.calls, req)
	return f.respond(len(f.calls), req)
}

func reply(status int, body string) HTTPResponse {
	return HTTPResponse{Status: status, Header: map[string][]string{"content-type": {"application/json"}}, Body: []byte(body)}
}

func replyHeader(status int, body string, header map[string][]string) HTTPResponse {
	r := reply(status, body)
	r.Header = header
	return r
}

func okSender(body string) *fakeSender {
	return &fakeSender{respond: func(int, HTTPRequest) (HTTPResponse, error) { return reply(200, body), nil }}
}

func newInterp(t *testing.T, def Definition, s Sender, sleeps *[]time.Duration) *Interpreter {
	t.Helper()
	opts := Options{
		Sender: s,
		Now:    func() time.Time { return time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC) },
		Sleep: func(_ context.Context, d time.Duration) error {
			if sleeps != nil {
				*sleeps = append(*sleeps, d)
			}
			return nil
		},
	}
	in, err := New(def, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return in
}

func baseDef(auth Auth, ops map[string]Operation) Definition {
	return Definition{
		Format:     Format,
		HTTP:       HTTP{BaseURL: "https://api.example.com/v1", Auth: auth},
		Operations: ops,
	}
}

func readOp(path string) Operation {
	return Operation{
		Purity: PurityRead, Safety: SafetyReadOnly,
		Request:  Request{Method: "GET", Path: path},
		Response: ResponseSpec{Select: "$.data"},
	}
}

func writeOp(safety string) Operation {
	op := Operation{
		Purity: PurityEffect, Safety: safety,
		Request:  Request{Method: "POST", Path: "/things"},
		Response: ResponseSpec{},
	}
	if safety == SafetyIdempotencyKey {
		op.IdempotencyHeader = "Idempotency-Key"
		op.IdempotencyKey = "{{$parameter.key}}"
	}
	return op
}

func asFailure(t *testing.T, err error) *CallError {
	t.Helper()
	var f *CallError
	if !errors.As(err, &f) {
		t.Fatalf("want *CallError, got %v", err)
	}
	return f
}

func TestAuthSchemes(t *testing.T) {
	creds := map[string]string{
		"apiKey": "KEY1", "username": "user", "password": "pass", "token": "tok",
		"tenant": "acme", "accessKeyId": "AKID", "secretAccessKey": "SECRET",
	}
	cases := []struct {
		name  string
		auth  Auth
		check func(t *testing.T, req HTTPRequest)
	}{
		{"NONE", Auth{Type: AuthNone}, func(t *testing.T, req HTTPRequest) {
			t.Helper()
			if len(req.Header["Authorization"]) != 0 {
				t.Errorf("NONE sent an Authorization header")
			}
		}},
		{"API_KEY header", Auth{Type: AuthAPIKey, In: "HEADER", Name: "X-Api-Key"}, func(t *testing.T, req HTTPRequest) {
			t.Helper()
			if got := req.Header["X-Api-Key"]; len(got) != 1 || got[0] != "KEY1" {
				t.Errorf("header = %v", got)
			}
		}},
		{"API_KEY query", Auth{Type: AuthAPIKey, In: "QUERY", Name: "api_key", Prefix: "p-"}, func(t *testing.T, req HTTPRequest) {
			t.Helper()
			u, _ := url.Parse(req.URL)
			if got := u.Query().Get("api_key"); got != "p-KEY1" {
				t.Errorf("query value = %q", got)
			}
		}},
		{"API_KEY cookie", Auth{Type: AuthAPIKey, In: "COOKIE", Name: "sid"}, func(t *testing.T, req HTTPRequest) {
			t.Helper()
			if got := req.Header["Cookie"]; len(got) != 1 || got[0] != "sid=KEY1" {
				t.Errorf("cookie = %v", got)
			}
		}},
		{"BASIC", Auth{Type: AuthBasic}, func(t *testing.T, req HTTPRequest) {
			t.Helper()
			want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
			if got := req.Header["Authorization"]; len(got) != 1 || got[0] != want {
				t.Errorf("authorization = %v", got)
			}
		}},
		{"BEARER", Auth{Type: AuthBearer}, func(t *testing.T, req HTTPRequest) {
			t.Helper()
			if got := req.Header["Authorization"]; len(got) != 1 || got[0] != "Bearer tok" {
				t.Errorf("authorization = %v", got)
			}
		}},
		{"CUSTOM_HEADERS", Auth{Type: AuthCustomHeaders, Headers: map[string]string{"X-Tenant": "{{$credentials.tenant}}"}}, func(t *testing.T, req HTTPRequest) {
			t.Helper()
			if got := req.Header["X-Tenant"]; len(got) != 1 || got[0] != "acme" {
				t.Errorf("tenant header = %v", got)
			}
		}},
		{"AWS_SIG_V4", Auth{Type: AuthAWSSigV4, Service: "s3", Region: "us-east-1"}, func(t *testing.T, req HTTPRequest) {
			t.Helper()
			got := req.Header["Authorization"]
			if len(got) != 1 || !strings.HasPrefix(got[0], "AWS4-HMAC-SHA256 Credential=AKID/20261014/us-east-1/s3/aws4_request") {
				t.Errorf("authorization = %v", got)
			}
			if d := req.Header["X-Amz-Date"]; len(d) != 1 || d[0] != "20261014T120000Z" {
				t.Errorf("x-amz-date = %v", d)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := okSender(`{"data":[]}`)
			in := newInterp(t, baseDef(tc.auth, map[string]Operation{"item.list": readOp("/items")}), s, nil)
			if _, err := in.Execute(t.Context(), Call{Operation: "item.list", Credentials: creds}, nil); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if len(s.calls) != 1 {
				t.Fatalf("sent %d requests", len(s.calls))
			}
			tc.check(t, s.calls[0])
		})
	}
}

func TestAWSSignatureDependsOnSecret(t *testing.T) {
	sign := func(secret string) string {
		s := okSender(`{"data":[]}`)
		in := newInterp(t, baseDef(Auth{Type: AuthAWSSigV4, Service: "s3", Region: "us-east-1"}, map[string]Operation{"item.list": readOp("/items")}), s, nil)
		_, _ = in.Execute(t.Context(), Call{Operation: "item.list", Credentials: map[string]string{"accessKeyId": "AKID", "secretAccessKey": secret}}, nil)
		return s.calls[0].Header["Authorization"][0]
	}
	a, b := sign("one"), sign("two")
	if a == b {
		t.Fatal("the signature does not depend on the secret")
	}
	if sign("one") != a {
		t.Fatal("the signature is not deterministic")
	}
}

func TestMissingCredentialFailsBeforeSending(t *testing.T) {
	s := okSender(`{"data":[]}`)
	in := newInterp(t, baseDef(Auth{Type: AuthBearer}, map[string]Operation{"item.list": readOp("/items")}), s, nil)
	_, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
	f := asFailure(t, err)
	if f.Class != ClassPermanent || f.Code != "AuthInvalid" {
		t.Fatalf("failure = %s/%s", f.Class, f.Code)
	}
	if len(s.calls) != 0 {
		t.Fatal("a request was sent without its credential")
	}
}

func TestCredentialsNeverInItems(t *testing.T) {
	const marker = "value-42-marker"
	s := okSender(`{"data":[{"id":1},{"id":2}]}`)
	in := newInterp(t, baseDef(Auth{Type: AuthBearer}, map[string]Operation{"item.list": readOp("/items")}), s, nil)
	res, err := in.Execute(t.Context(), Call{Operation: "item.list", Credentials: map[string]string{"token": marker}}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for i, item := range res.Items {
		b, _ := jsonvalue.Canonical(item)
		if strings.Contains(string(b), marker) {
			t.Errorf("item %d carries the credential", i)
		}
	}

	bad := readOp("/items")
	bad.Response.Select = "$.data.$credentials"
	if err := Validate(baseDef(Auth{Type: AuthBearer}, map[string]Operation{"x.y": bad})); err == nil {
		t.Error("a response mapping that reads $credentials was accepted")
	}
	leaky := readOp("/items/{{$credentials.token}}")
	if err := Validate(baseDef(Auth{Type: AuthBearer}, map[string]Operation{"x.y": leaky})); err == nil {
		t.Error("a request path that reads $credentials was accepted")
	}

	fail := &fakeSender{respond: func(int, HTTPRequest) (HTTPResponse, error) {
		return reply(400, `{"message":"bad"}`), nil
	}}
	in = newInterp(t, baseDef(Auth{Type: AuthBearer}, map[string]Operation{"item.list": readOp("/items")}), fail, nil)
	_, err = in.Execute(t.Context(), Call{Operation: "item.list", Credentials: map[string]string{"token": marker}}, nil)
	if err == nil || strings.Contains(err.Error(), marker) {
		t.Errorf("failure text leaks the credential: %v", err)
	}
}

const pagedTotal = 25

// pagedProvider serves 25 items in the five paging styles of ADR-0030 §6.
func pagedProvider(n int, req HTTPRequest) (HTTPResponse, error) {
	u, err := url.Parse(req.URL)
	if err != nil {
		return HTTPResponse{}, err
	}
	q := u.Query()
	atoi := func(s string, def int) int {
		var v int
		if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
			return def
		}
		return v
	}
	const total, size = pagedTotal, 10
	switch u.Path {
	case "/v1/items":
		off := atoi(q.Get("offset"), 0)
		lim := atoi(q.Get("limit"), size)
		return reply(200, `{"data":`+itemsJSON(off, lim)+`}`), nil
	case "/v1/pages":
		page := atoi(q.Get("page"), 1)
		return reply(200, `{"data":`+itemsJSON((page-1)*size, size)+`}`), nil
	case "/v1/cursor":
		start := atoi(q.Get("cursor"), 0)
		body := `{"data":` + itemsJSON(start, size)
		if start+size < total {
			body += fmt.Sprintf(`,"next":"%d"`, start+size)
		}
		return reply(200, body+`}`), nil
	case "/v1/link":
		start := atoi(q.Get("start"), 0)
		hdr := map[string][]string{}
		if start+size < total {
			hdr["link"] = []string{fmt.Sprintf(`<https://api.example.com/v1/link?start=%d>; rel="next", <https://api.example.com/v1/x>; rel="prev"`, start+size)}
		}
		return replyHeader(200, itemsJSON(start, size), hdr), nil
	case "/v1/next":
		start := atoi(q.Get("start"), 0)
		body := `{"data":` + itemsJSON(start, size)
		if start+size < total {
			body += fmt.Sprintf(`,"nextUrl":"https://api.example.com/v1/next?start=%d"`, start+size)
		}
		return reply(200, body+`}`), nil
	}
	return reply(404, `{}`), nil
}

func itemsJSON(off, lim int) string {
	var b strings.Builder
	b.WriteString("[")
	first := true
	for i := off; i < off+lim && i < pagedTotal; i++ {
		if !first {
			b.WriteString(",")
		}
		first = false
		fmt.Fprintf(&b, `{"id":%d}`, i)
	}
	b.WriteString("]")
	return b.String()
}

func TestPagination(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		pg    Pagination
		pages int
	}{
		{"OFFSET", "/items", Pagination{Strategy: "OFFSET", Param: "offset", LimitParam: "limit", PageSize: 10}, 3},
		{"PAGE", "/pages", Pagination{Strategy: "PAGE", Param: "page", Start: 1}, 4},
		{"CURSOR", "/cursor", Pagination{Strategy: "CURSOR", Param: "cursor", From: "PATH", Path: "$.next"}, 3},
		{"LINK_HEADER", "/link", Pagination{Strategy: "LINK_HEADER"}, 3},
		{"NEXT_URL", "/next", Pagination{Strategy: "NEXT_URL", Path: "$.nextUrl"}, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := readOp(tc.path)
			if tc.name == "LINK_HEADER" {
				op.Response.Select = ""
			}
			op.Pagination = &tc.pg
			s := &fakeSender{respond: pagedProvider}
			in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), s, nil)
			res, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if len(res.Items) != 25 {
				t.Fatalf("got %d items, want 25", len(res.Items))
			}
			for i, item := range res.Items {
				id, _ := item.Get("id")
				if n, _ := id.Int64(); n != int64(i) {
					t.Fatalf("item %d has id %d", i, n)
				}
			}
			if res.Pages != tc.pages {
				t.Errorf("pages = %d, want %d", res.Pages, tc.pages)
			}
		})
	}
}

func TestPaginationMaxPages(t *testing.T) {
	op := readOp("/pages")
	op.Pagination = &Pagination{Strategy: "PAGE", Param: "page", Start: 1, MaxPages: 2}
	in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), &fakeSender{respond: pagedProvider}, nil)
	_, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
	if f := asFailure(t, err); f.Class != ClassPermanent {
		t.Fatalf("class = %s, want PERMANENT", f.Class)
	}

	op.Pagination.OnMaxPages = "TRUNCATE"
	in = newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), &fakeSender{respond: pagedProvider}, nil)
	res, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
	if err != nil {
		t.Fatalf("truncated walk failed: %v", err)
	}
	if len(res.Warnings) != 1 || len(res.Items) != 20 {
		t.Fatalf("warnings=%v items=%d", res.Warnings, len(res.Items))
	}
}

func TestPaginationStreamsWithoutGrowingMemory(t *testing.T) {
	const pages = 10000
	op := readOp("/pages")
	op.Pagination = &Pagination{Strategy: "PAGE", Param: "page", Start: 1, MaxPages: pages}
	endless := &fakeSender{respond: func(n int, _ HTTPRequest) (HTTPResponse, error) {
		return reply(200, `{"data":[{"n":1}]}`), nil
	}}
	in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), endless, nil)
	count := 0
	res, err := in.Execute(t.Context(), Call{Operation: "item.list"}, func(items []jsonvalue.Value) error {
		count += len(items)
		return nil
	})
	if err == nil {
		t.Fatalf("an endless walk must stop at maxPages with a failure, got %+v", res)
	}
	if res.Items != nil {
		t.Fatal("a streamed call kept its items in memory")
	}
	if count != pages {
		t.Fatalf("sink received %d items, want %d", count, pages)
	}
}

func TestPaginationRefusedOnEffect(t *testing.T) {
	op := writeOp(SafetyUnsafe)
	op.Pagination = &Pagination{Strategy: "PAGE", Param: "page"}
	if err := Validate(baseDef(Auth{Type: AuthNone}, map[string]Operation{"thing.create": op})); err == nil {
		t.Fatal("pagination on an EFFECT operation was accepted")
	}
}

func TestRetryClassification(t *testing.T) {
	t.Run("READ_ONLY retries 503 and honors Retry-After", func(t *testing.T) {
		var sleeps []time.Duration
		s := &fakeSender{respond: func(n int, _ HTTPRequest) (HTTPResponse, error) {
			if n < 3 {
				return replyHeader(429, `{}`, map[string][]string{"retry-after": {"2"}}), nil
			}
			return reply(200, `{"data":[{"id":1}]}`), nil
		}}
		in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": readOp("/items")}), s, &sleeps)
		res, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
		if err != nil || len(res.Items) != 1 {
			t.Fatalf("err=%v items=%d", err, len(res.Items))
		}
		if len(s.calls) != 3 || len(sleeps) != 2 || sleeps[0] != 2*time.Second {
			t.Fatalf("calls=%d sleeps=%v", len(s.calls), sleeps)
		}
	})

	t.Run("UNSAFE is never retried, even on 429", func(t *testing.T) {
		s := &fakeSender{respond: func(int, HTTPRequest) (HTTPResponse, error) {
			return reply(429, `{}`), nil
		}}
		in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"thing.create": writeOp(SafetyUnsafe)}), s, nil)
		_, err := in.Execute(t.Context(), Call{Operation: "thing.create"}, nil)
		f := asFailure(t, err)
		if f.Class != ClassTransient || f.Code != "RateLimited" || len(s.calls) != 1 {
			t.Fatalf("failure=%s/%s calls=%d", f.Class, f.Code, len(s.calls))
		}
	})

	t.Run("IDEMPOTENCY_KEY repeats with the same key", func(t *testing.T) {
		s := &fakeSender{respond: func(n int, _ HTTPRequest) (HTTPResponse, error) {
			if n == 1 {
				return reply(503, `{}`), nil
			}
			return reply(201, `{"id":"x"}`), nil
		}}
		def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"thing.create": writeOp(SafetyIdempotencyKey)})
		in := newInterp(t, def, s, nil)
		_, err := in.Execute(t.Context(), Call{Operation: "thing.create", Parameters: map[string]jsonvalue.Value{"key": jsonvalue.String("k-1")}}, nil)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if len(s.calls) != 2 {
			t.Fatalf("calls = %d, want 2", len(s.calls))
		}
		for i, c := range s.calls {
			if got := c.Header["Idempotency-Key"]; len(got) != 1 || got[0] != "k-1" {
				t.Errorf("call %d key = %v", i, got)
			}
		}
	})

	t.Run("maxAttempts bounds the retries", func(t *testing.T) {
		op := readOp("/items")
		op.MaxAttempts = 2
		s := &fakeSender{respond: func(int, HTTPRequest) (HTTPResponse, error) { return reply(503, `{}`), nil }}
		in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), s, nil)
		_, _ = in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
		if len(s.calls) != 2 {
			t.Fatalf("calls = %d, want 2", len(s.calls))
		}
	})
}

func TestAmbiguousWriteClassification(t *testing.T) {
	unsafeRule := writeOp(SafetyUnsafe)
	unsafeRule.Response.Errors = []ErrorRule{{Status: []int{503}, Class: "PERMANENT", Code: "ServiceUnavailable"}}
	connReset := errors.New("connection reset by peer")

	type want struct {
		class  Class
		action Action
		calls  int
		ambig  bool
	}
	cases := []struct {
		name  string
		op    Operation
		fault func(n int) (HTTPResponse, error)
		want  want
	}{
		{"UNSAFE 5xx is UNCERTAIN, never resent", writeOp(SafetyUnsafe),
			func(int) (HTTPResponse, error) { return reply(500, `{}`), nil },
			want{ClassUncertain, ActionSuspend, 1, true}},
		{"UNSAFE 5xx ignores a PERMANENT rule", unsafeRule,
			func(int) (HTTPResponse, error) { return reply(503, `{}`), nil },
			want{ClassUncertain, ActionSuspend, 1, true}},
		{"UNSAFE connection cut after sending is UNCERTAIN", writeOp(SafetyUnsafe),
			func(int) (HTTPResponse, error) { return HTTPResponse{}, connReset },
			want{ClassUncertain, ActionSuspend, 1, true}},
		{"UNSAFE not-sent error is safe to report as TRANSIENT", writeOp(SafetyUnsafe),
			func(int) (HTTPResponse, error) { return HTTPResponse{}, fmt.Errorf("%w: dns", ErrNotSent) },
			want{ClassTransient, ActionNone, 1, false}},
		{"RECONCILE 5xx asks for verification before resending", writeOp(SafetyReconcile),
			func(int) (HTTPResponse, error) { return reply(502, `{}`), nil },
			want{ClassTransient, ActionVerify, 1, true}},
		{"RECONCILE connection cut asks for verification", writeOp(SafetyReconcile),
			func(int) (HTTPResponse, error) { return HTTPResponse{}, connReset },
			want{ClassTransient, ActionVerify, 1, true}},
		{"IDEMPOTENCY_KEY 5xx is TRANSIENT and retried", writeOp(SafetyIdempotencyKey),
			func(int) (HTTPResponse, error) { return reply(500, `{}`), nil },
			want{ClassTransient, ActionNone, 3, false}},
		{"READ_ONLY 5xx is TRANSIENT and retried", readOp("/items"),
			func(int) (HTTPResponse, error) { return reply(500, `{}`), nil },
			want{ClassTransient, ActionNone, 3, false}},
		{"429 is TRANSIENT and not applied on UNSAFE", writeOp(SafetyUnsafe),
			func(int) (HTTPResponse, error) { return reply(429, `{}`), nil },
			want{ClassTransient, ActionNone, 1, false}},
		{"429 is TRANSIENT and not applied on RECONCILE", writeOp(SafetyReconcile),
			func(int) (HTTPResponse, error) { return reply(429, `{}`), nil },
			want{ClassTransient, ActionNone, 1, false}},
		{"egress refusal is PERMANENT EgressDenied", writeOp(SafetyUnsafe),
			func(int) (HTTPResponse, error) { return HTTPResponse{}, fmt.Errorf("%w: private", ErrEgressDenied) },
			want{ClassPermanent, ActionNone, 1, false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeSender{respond: func(n int, _ HTTPRequest) (HTTPResponse, error) { return tc.fault(n) }}
			def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"x.y": tc.op})
			in := newInterp(t, def, s, nil)
			call := Call{Operation: "x.y", Parameters: map[string]jsonvalue.Value{"key": jsonvalue.String("k")}}
			_, err := in.Execute(t.Context(), call, nil)
			f := asFailure(t, err)
			if f.Class != tc.want.class || f.Action != tc.want.action || f.Ambiguous != tc.want.ambig {
				t.Errorf("failure class=%s action=%s ambiguous=%v; want class=%s action=%s ambiguous=%v",
					f.Class, f.Action, f.Ambiguous, tc.want.class, tc.want.action, tc.want.ambig)
			}
			if len(s.calls) != tc.want.calls {
				t.Errorf("sent %d requests, want %d", len(s.calls), tc.want.calls)
			}
		})
	}
}

func TestUncertainClassOnlyOnEffect(t *testing.T) {
	read := readOp("/items")
	read.Response.Errors = []ErrorRule{{Status: []int{500}, Class: "UNCERTAIN"}}
	if err := Validate(baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": read})); err == nil {
		t.Fatal("UNCERTAIN was accepted on a READ operation")
	}
	effect := writeOp(SafetyUnsafe)
	effect.Response.Errors = []ErrorRule{{Status: []int{500}, Class: "UNCERTAIN"}}
	if err := Validate(baseDef(Auth{Type: AuthNone}, map[string]Operation{"thing.create": effect})); err != nil {
		t.Fatalf("UNCERTAIN on an EFFECT operation was refused: %v", err)
	}
}

func TestErrorCodesClosedList(t *testing.T) {
	if len(Codes) != 14 {
		t.Fatalf("the closed list has %d codes, want 14", len(Codes))
	}
	for _, code := range Codes {
		op := readOp("/items")
		op.Response.Errors = []ErrorRule{{Status: []int{404}, Class: "PERMANENT", Code: code}}
		if err := Validate(baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op})); err != nil {
			t.Errorf("code %s refused: %v", code, err)
		}
	}
	op := readOp("/items")
	op.Response.Errors = []ErrorRule{{Status: []int{404}, Class: "PERMANENT", Code: "ConnectorRevoked"}}
	if err := Validate(baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op})); err == nil {
		t.Fatal("a host-raised code was accepted in a connector mapping")
	}
}

func TestValidateRefusesUnsupportedSchemes(t *testing.T) {
	for _, typ := range []string{AuthOAuth2, AuthSessionToken, "MAGIC"} {
		if err := Validate(baseDef(Auth{Type: typ}, map[string]Operation{"item.list": readOp("/items")})); err == nil {
			t.Errorf("scheme %s was accepted", typ)
		}
	}
}
