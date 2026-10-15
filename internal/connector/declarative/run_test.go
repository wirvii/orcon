// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestNewRefusesMissingSenderAndInvalidDefinition(t *testing.T) {
	def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": readOp("/items")})
	if _, err := New(def, Options{}); err == nil || !strings.Contains(err.Error(), "a sender is required") {
		t.Fatalf("New without a sender: %v", err)
	}
	bad := def
	bad.Format = "some-other-format"
	if _, err := New(bad, Options{Sender: okSender(`{}`)}); err == nil {
		t.Fatal("New accepted a definition of another format")
	}
}

func TestNewDefaultsClockAndSleep(t *testing.T) {
	// No Now is given: AWS signing must still run with the real clock.
	def := baseDef(Auth{Type: AuthAWSSigV4, Service: "s3", Region: "us-east-1"}, map[string]Operation{"item.list": readOp("/items")})
	s := okSender(`{"data":[]}`)
	in, err := New(def, Options{Sender: s})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	creds := map[string]string{"accessKeyId": "AKID", "secretAccessKey": "SECRET"}
	if _, err := in.Execute(t.Context(), Call{Operation: "item.list", Credentials: creds}, nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := s.calls[0].Header["Authorization"]; len(got) != 1 || !strings.HasPrefix(got[0], "AWS4-HMAC-SHA256 Credential=AKID/") {
		t.Fatalf("authorization = %v", got)
	}
}

func TestSleepCtxHonorsCancellation(t *testing.T) {
	if err := sleepCtx(t.Context(), time.Millisecond); err != nil {
		t.Fatalf("a short wait failed: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := sleepCtx(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("a canceled wait returned %v", err)
	}
}

func TestExecuteUnknownOperationSendsNothing(t *testing.T) {
	s := okSender(`{}`)
	in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": readOp("/items")}), s, nil)
	_, err := in.Execute(t.Context(), Call{Operation: "item.delete"}, nil)
	f := asFailure(t, err)
	if f.Class != ClassPermanent || f.Code != "ValidationFailed" || len(s.calls) != 0 {
		t.Fatalf("failure=%s/%s calls=%d", f.Class, f.Code, len(s.calls))
	}
}

func TestWithSenderReplacesOnlyTheSender(t *testing.T) {
	first := okSender(`{"data":[{"id":1}]}`)
	second := okSender(`{"data":[{"id":2}]}`)
	in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": readOp("/items")}), first, nil)
	other, err := in.WithSender(second)
	if err != nil {
		t.Fatalf("WithSender: %v", err)
	}
	res, err := other.Execute(t.Context(), Call{Operation: "item.list"}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	id, _ := res.Items[0].Get("id")
	if n, _ := id.Int64(); n != 2 || len(first.calls) != 0 || len(second.calls) != 1 {
		t.Fatalf("id=%d first=%d second=%d", n, len(first.calls), len(second.calls))
	}
}

func TestMissingCredentialFieldsFailBeforeSending(t *testing.T) {
	cases := []struct {
		name  string
		auth  Auth
		creds map[string]string
		code  string
	}{
		{"BASIC without password", Auth{Type: AuthBasic}, map[string]string{"username": "u"}, "AuthInvalid"},
		{"API_KEY with an empty key", Auth{Type: AuthAPIKey, In: "HEADER", Name: "X-Key"}, map[string]string{"apiKey": ""}, "AuthInvalid"},
		{"AWS without secret", Auth{Type: AuthAWSSigV4, Service: "s3", Region: "r"}, map[string]string{"accessKeyId": "AKID"}, "AuthInvalid"},
		{"CUSTOM_HEADERS field absent", Auth{Type: AuthCustomHeaders, Headers: map[string]string{"X-T": "{{$credentials.tenant}}"}}, map[string]string{}, "ValidationFailed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := okSender(`{"data":[]}`)
			in := newInterp(t, baseDef(tc.auth, map[string]Operation{"item.list": readOp("/items")}), s, nil)
			_, err := in.Execute(t.Context(), Call{Operation: "item.list", Credentials: tc.creds}, nil)
			f := asFailure(t, err)
			if f.Class != ClassPermanent || f.Code != tc.code {
				t.Fatalf("failure = %s/%s, want PERMANENT/%s", f.Class, f.Code, tc.code)
			}
			if len(s.calls) != 0 {
				t.Fatal("a request was sent without its credential")
			}
		})
	}
}

// relativeProvider answers the paging links with paths relative to the host,
// as many providers do, so the interpreter must resolve them against the base.
func relativeProvider(linkHeader bool) func(int, HTTPRequest) (HTTPResponse, error) {
	return func(_ int, req HTTPRequest) (HTTPResponse, error) {
		u, err := url.Parse(req.URL)
		if err != nil {
			return HTTPResponse{}, err
		}
		if u.Query().Get("start") == "" {
			if linkHeader {
				hdr := map[string][]string{"link": {`</v1/link?start=1>; rel="next"`}}
				return replyHeader(200, `[{"id":0}]`, hdr), nil
			}
			return reply(200, `{"data":[{"id":0}],"nextUrl":"/v1/next?start=1"}`), nil
		}
		return reply(200, `{"data":[{"id":1}]}`), nil
	}
}

func TestPagingLinksResolveAgainstTheBaseURL(t *testing.T) {
	t.Run("NEXT_URL relative", func(t *testing.T) {
		op := readOp("/next")
		op.Pagination = &Pagination{Strategy: "NEXT_URL", Path: "$.nextUrl"}
		s := &fakeSender{respond: relativeProvider(false)}
		in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), s, nil)
		res, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
		if err != nil || len(res.Items) != 2 || res.Pages != 2 {
			t.Fatalf("err=%v items=%d pages=%d", err, len(res.Items), res.Pages)
		}
		if got := s.calls[1].URL; got != "https://api.example.com/v1/next?start=1" {
			t.Fatalf("second page = %s", got)
		}
	})
	t.Run("LINK_HEADER relative", func(t *testing.T) {
		op := readOp("/link")
		op.Response.Select = ""
		op.Pagination = &Pagination{Strategy: "LINK_HEADER"}
		s := &fakeSender{respond: relativeProvider(true)}
		in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), s, nil)
		res, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
		if err != nil || len(res.Items) != 2 || res.Pages != 2 {
			t.Fatalf("err=%v items=%d pages=%d", err, len(res.Items), res.Pages)
		}
		if got := s.calls[1].URL; got != "https://api.example.com/v1/link?start=1" {
			t.Fatalf("second page = %s", got)
		}
	})
}

func TestMalformedNextURLFailsBeforeSending(t *testing.T) {
	op := readOp("/next")
	op.Pagination = &Pagination{Strategy: "NEXT_URL", Path: "$.nextUrl"}
	s := &fakeSender{respond: func(int, HTTPRequest) (HTTPResponse, error) {
		return reply(200, `{"data":[{"id":0}],"nextUrl":"http://[::1"}`), nil
	}}
	in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), s, nil)
	_, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
	f := asFailure(t, err)
	if f.Class != ClassPermanent || f.Code != "ValidationFailed" || len(s.calls) != 1 {
		t.Fatalf("failure=%s/%s calls=%d", f.Class, f.Code, len(s.calls))
	}
}

func TestCursorReadsHeaderAndNumericPathCursors(t *testing.T) {
	t.Run("HEADER", func(t *testing.T) {
		op := readOp("/cursor")
		op.Pagination = &Pagination{Strategy: "CURSOR", Param: "cursor", From: "HEADER", Header: "X-Next-Cursor"}
		s := &fakeSender{respond: func(_ int, req HTTPRequest) (HTTPResponse, error) {
			u, _ := url.Parse(req.URL)
			switch u.Query().Get("cursor") {
			case "":
				return replyHeader(200, `{"data":[{"id":0},{"id":1}]}`, map[string][]string{"x-next-cursor": {"c2"}}), nil
			case "c2":
				return reply(200, `{"data":[{"id":2}]}`), nil
			}
			return reply(400, `{}`), nil
		}}
		in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), s, nil)
		res, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
		if err != nil || len(res.Items) != 3 || res.Pages != 2 {
			t.Fatalf("err=%v items=%d pages=%d", err, len(res.Items), res.Pages)
		}
	})
	t.Run("PATH numeric", func(t *testing.T) {
		op := readOp("/cursor")
		op.Pagination = &Pagination{Strategy: "CURSOR", Param: "cursor", From: "PATH", Path: "$.next"}
		s := &fakeSender{respond: func(_ int, req HTTPRequest) (HTTPResponse, error) {
			u, _ := url.Parse(req.URL)
			if u.Query().Get("cursor") == "" {
				return reply(200, `{"data":[{"id":0}],"next":10}`), nil
			}
			return reply(200, `{"data":[{"id":1}]}`), nil
		}}
		in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), s, nil)
		res, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
		if err != nil || len(res.Items) != 2 || res.Pages != 2 {
			t.Fatalf("err=%v items=%d pages=%d", err, len(res.Items), res.Pages)
		}
		if got := s.calls[1].URL; !strings.HasSuffix(got, "/cursor?cursor=10") {
			t.Fatalf("second page = %s", got)
		}
	})
}

func TestPageNumberingAndOffsetWithoutPageSize(t *testing.T) {
	t.Run("PAGE starting at zero begins at page 1", func(t *testing.T) {
		op := readOp("/pages")
		op.Pagination = &Pagination{Strategy: "PAGE", Param: "page", Start: 0}
		s := &fakeSender{respond: pagedProvider}
		in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), s, nil)
		if _, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if got := s.calls[0].URL; !strings.HasSuffix(got, "page=1") {
			t.Fatalf("first page = %s", got)
		}
	})
	t.Run("OFFSET without a page size ends on an empty page", func(t *testing.T) {
		op := readOp("/items")
		op.Pagination = &Pagination{Strategy: "OFFSET", Param: "offset"}
		s := &fakeSender{respond: pagedProvider}
		in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), s, nil)
		res, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
		if err != nil || len(res.Items) != pagedTotal || res.Pages != 4 {
			t.Fatalf("err=%v items=%d pages=%d", err, len(res.Items), res.Pages)
		}
	})
}

func TestSinkFailureStopsTheWalk(t *testing.T) {
	errFull := errors.New("sink is full")
	op := readOp("/pages")
	op.Pagination = &Pagination{Strategy: "PAGE", Param: "page", Start: 1}
	s := &fakeSender{respond: pagedProvider}
	in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), s, nil)
	_, err := in.Execute(t.Context(), Call{Operation: "item.list"}, func([]jsonvalue.Value) error { return errFull })
	if !errors.Is(err, errFull) || len(s.calls) != 1 {
		t.Fatalf("err=%v calls=%d", err, len(s.calls))
	}
}

func TestResponseMapping(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		body      string
		selectP   string
		empty     string
		wantCount int
		wantFirst string
	}{
		{"array under select", 200, `{"data":[{"id":1},{"id":2}]}`, "$.data", "", 2, `{"id":1}`},
		{"object becomes one item", 200, `{"one":{"id":9}}`, "$.one", "", 1, `{"id":9}`},
		{"whole body when select is empty", 200, `[{"id":3}]`, "", "", 1, `{"id":3}`},
		{"empty array with NO_ITEMS", 200, `{"data":[]}`, "$.data", "NO_ITEMS", 0, ""},
		{"empty array with ONE_EMPTY_ITEM", 200, `{"data":[]}`, "$.data", "ONE_EMPTY_ITEM", 1, `{}`},
		{"missing path with ONE_EMPTY_ITEM", 200, `{"other":1}`, "$.data", "ONE_EMPTY_ITEM", 1, `{}`},
		{"no content yields no items", 204, ``, "$.data", "", 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := readOp("/items")
			op.Response = ResponseSpec{Select: tc.selectP, Empty: tc.empty}
			s := &fakeSender{respond: func(int, HTTPRequest) (HTTPResponse, error) { return reply(tc.status, tc.body), nil }}
			in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": op}), s, nil)
			res, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if len(res.Items) != tc.wantCount {
				t.Fatalf("items = %d, want %d", len(res.Items), tc.wantCount)
			}
			if tc.wantFirst != "" {
				got, _ := jsonvalue.Canonical(res.Items[0])
				if string(got) != tc.wantFirst {
					t.Fatalf("first item = %s, want %s", got, tc.wantFirst)
				}
			}
		})
	}
}

func TestNonJSONSuccessBodyIsPermanent(t *testing.T) {
	s := &fakeSender{respond: func(int, HTTPRequest) (HTTPResponse, error) { return reply(200, `<html>`), nil }}
	in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": readOp("/items")}), s, nil)
	_, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
	f := asFailure(t, err)
	if f.Class != ClassPermanent || f.Code != "ProviderError" || f.Status != 200 || len(s.calls) != 1 {
		t.Fatalf("failure=%s/%s status=%d calls=%d", f.Class, f.Code, f.Status, len(s.calls))
	}
}

func TestRetryBackoffWithoutRetryAfter(t *testing.T) {
	var sleeps []time.Duration
	s := &fakeSender{respond: func(n int, _ HTTPRequest) (HTTPResponse, error) {
		if n < 3 {
			return reply(503, `{}`), nil
		}
		return reply(200, `{"data":[]}`), nil
	}}
	in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": readOp("/items")}), s, &sleeps)
	if _, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(sleeps) != 2 || sleeps[0] != 200*time.Millisecond || sleeps[1] != 400*time.Millisecond {
		t.Fatalf("sleeps = %v, want 200ms then 400ms", sleeps)
	}
}

func TestRetryStopsWhenTheWaitIsInterrupted(t *testing.T) {
	s := &fakeSender{respond: func(int, HTTPRequest) (HTTPResponse, error) { return reply(503, `{}`), nil }}
	in, err := New(baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": readOp("/items")}), Options{
		Sender: s,
		Sleep:  func(context.Context, time.Duration) error { return context.Canceled },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = in.Execute(t.Context(), Call{Operation: "item.list"}, nil)
	f := asFailure(t, err)
	if f.Class != ClassTransient || f.Code != "Timeout" || len(s.calls) != 1 {
		t.Fatalf("failure=%s/%s calls=%d", f.Class, f.Code, len(s.calls))
	}
}

func TestTransportErrorClassification(t *testing.T) {
	netErr := errors.New("connection refused")
	cases := []struct {
		name      string
		op        Operation
		err       error
		class     Class
		code      string
		ambiguous bool
		calls     int
	}{
		{"READ network error is retried", readOp("/items"), netErr, ClassTransient, "NetworkError", false, 3},
		{"READ deadline is a Timeout", readOp("/items"), context.DeadlineExceeded, ClassTransient, "Timeout", false, 3},
		{"UNSAFE deadline is UNCERTAIN", writeOp(SafetyUnsafe), context.DeadlineExceeded, ClassUncertain, "Timeout", true, 1},
		{"IDEMPOTENCY_KEY network error is retried", writeOp(SafetyIdempotencyKey), netErr, ClassTransient, "NetworkError", false, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeSender{respond: func(int, HTTPRequest) (HTTPResponse, error) { return HTTPResponse{}, tc.err }}
			in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"x.y": tc.op}), s, nil)
			call := Call{Operation: "x.y", Parameters: map[string]jsonvalue.Value{"key": jsonvalue.String("k")}}
			_, err := in.Execute(t.Context(), call, nil)
			f := asFailure(t, err)
			if f.Class != tc.class || f.Code != tc.code || f.Ambiguous != tc.ambiguous || len(s.calls) != tc.calls {
				t.Fatalf("class=%s code=%s ambiguous=%v calls=%d", f.Class, f.Code, f.Ambiguous, len(s.calls))
			}
		})
	}
}

func TestRequestBuildingRendersQueryHeadersAndJSONBody(t *testing.T) {
	op := Operation{
		Purity: PurityEffect, Safety: SafetyUnsafe,
		Request: Request{
			Method: "POST",
			Path:   "/things/{{$parameter.id}}",
			Query:  map[string]string{"expand": "{{$parameter.expand}}"},
			// The operation header overrides the connector-wide one.
			Headers: map[string]string{"X-Version": "2"},
			Body: &Body{Type: "JSON", Fields: map[string]string{
				"name":  "{{$parameter.name}}",
				"count": "{{$parameter.count}}",
			}},
		},
		Response: ResponseSpec{},
	}
	def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"thing.create": op})
	def.HTTP.BaseURL = "https://api.example.com/v1/"
	def.HTTP.Headers = map[string]string{"X-Version": "1"}
	s := okSender(`{}`)
	in := newInterp(t, def, s, nil)
	params := map[string]jsonvalue.Value{
		"id": jsonvalue.String("42"), "expand": jsonvalue.String("owner"),
		"name": jsonvalue.String("Ana"), "count": jsonvalue.Int(3),
	}
	if _, err := in.Execute(t.Context(), Call{Operation: "thing.create", Parameters: params}, nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	req := s.calls[0]
	if req.URL != "https://api.example.com/v1/things/42?expand=owner" {
		t.Errorf("url = %s", req.URL)
	}
	if got := req.Header["X-Version"]; len(got) != 1 || got[0] != "2" {
		t.Errorf("X-Version = %v, want only the operation value", got)
	}
	if got := req.Header["Content-Type"]; len(got) != 1 || got[0] != "application/json" {
		t.Errorf("content type = %v", got)
	}
	if string(req.Body) != `{"count":3,"name":"Ana"}` {
		t.Errorf("body = %s", req.Body)
	}
}

func TestFormBodyIsEncoded(t *testing.T) {
	op := Operation{
		Purity: PurityEffect, Safety: SafetyUnsafe,
		Request: Request{
			Method: "POST", Path: "/forms",
			Body: &Body{Type: "FORM", Fields: map[string]string{"b": "{{$parameter.b}}", "a": "x y"}},
		},
	}
	s := okSender(`{}`)
	in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"form.submit": op}), s, nil)
	if _, err := in.Execute(t.Context(), Call{Operation: "form.submit", Parameters: map[string]jsonvalue.Value{"b": jsonvalue.String("two")}}, nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	req := s.calls[0]
	if string(req.Body) != "a=x+y&b=two" {
		t.Errorf("body = %s", req.Body)
	}
	if got := req.Header["Content-Type"]; len(got) != 1 || got[0] != "application/x-www-form-urlencoded" {
		t.Errorf("content type = %v", got)
	}
}

func TestRenderFailuresSendNothing(t *testing.T) {
	obj, _ := jsonvalue.Object(jsonvalue.Member{Key: "k", Value: jsonvalue.Int(1)})
	cases := []struct {
		name   string
		path   string
		query  map[string]string
		params map[string]jsonvalue.Value
	}{
		{"path parameter missing", "/items/{{$parameter.id}}", nil, nil},
		{"unknown placeholder", "/items/{{$env.HOME}}", nil, nil},
		{"unclosed placeholder", "/items/{{$parameter.id", nil, nil},
		{"object used as text", "/items", map[string]string{"q": "{{$parameter.obj}}"}, map[string]jsonvalue.Value{"obj": obj}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := Operation{
				Purity: PurityRead, Safety: SafetyReadOnly,
				Request:  Request{Method: "GET", Path: tc.path, Query: tc.query},
				Response: ResponseSpec{Select: "$.data"},
			}
			s := okSender(`{"data":[]}`)
			in := newInterp(t, baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.get": op}), s, nil)
			_, err := in.Execute(t.Context(), Call{Operation: "item.get", Parameters: tc.params}, nil)
			f := asFailure(t, err)
			if f.Class != ClassPermanent || f.Code != "ValidationFailed" || len(s.calls) != 0 {
				t.Fatalf("failure=%s/%s calls=%d", f.Class, f.Code, len(s.calls))
			}
		})
	}
}

func TestBaseURLTrailingSlashIsNotDoubled(t *testing.T) {
	def := baseDef(Auth{Type: AuthNone}, map[string]Operation{"item.list": readOp("/items")})
	def.HTTP.BaseURL = "https://api.example.com/v1/"
	s := okSender(`{"data":[]}`)
	in := newInterp(t, def, s, nil)
	if _, err := in.Execute(t.Context(), Call{Operation: "item.list"}, nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := s.calls[0].URL; got != "https://api.example.com/v1/items" {
		t.Fatalf("url = %s", got)
	}
}

func TestRetryAfterReadsSecondsOnly(t *testing.T) {
	cases := []struct {
		name   string
		header map[string][]string
		want   time.Duration
	}{
		{"seconds", map[string][]string{"Retry-After": {"7"}}, 7 * time.Second},
		{"surrounding spaces", map[string][]string{"retry-after": {" 3 "}}, 3 * time.Second},
		{"HTTP date is not read", map[string][]string{"retry-after": {"Wed, 21 Oct 2026 07:28:00 GMT"}}, 0},
		{"negative is ignored", map[string][]string{"retry-after": {"-1"}}, 0},
		{"absent", map[string][]string{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := retryAfterOf(HTTPResponse{Header: tc.header}); got != tc.want {
				t.Fatalf("retryAfterOf = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNextLinkPicksTheNextRelation(t *testing.T) {
	cases := []struct {
		name   string
		header []string
		want   string
	}{
		{"quoted next among others", []string{`<https://x/p?s=2>; rel="next", <https://x/p?s=0>; rel="prev"`}, "https://x/p?s=2"},
		{"unquoted next", []string{`<https://x/p?s=3>; rel=next`}, "https://x/p?s=3"},
		{"relation name is case-insensitive", []string{`<https://x/p?s=4>; REL="NEXT"`}, "https://x/p?s=4"},
		{"second header value", []string{`<https://x/a>; rel="prev"`, `<https://x/b>; rel="next"`}, "https://x/b"},
		{"no next relation", []string{`<https://x/a>; rel="prev"`}, ""},
		{"target without brackets", []string{`https://x/a; rel="next"`}, ""},
		{"segment without parameters", []string{`garbage`}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nextLink(tc.header); got != tc.want {
				t.Fatalf("nextLink = %q, want %q", got, tc.want)
			}
		})
	}
}
