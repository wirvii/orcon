// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/connector/limit"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
)

func reply(status int, contentType, body string) declarative.HTTPResponse {
	h := map[string][]string{}
	if contentType != "" {
		h["content-type"] = []string{contentType}
	}
	return declarative.HTTPResponse{Status: status, Header: h, Body: []byte(body)}
}

// runGet runs a GET of the node with the given parameters against one canned response.
func runGet(t *testing.T, params string, resp declarative.HTTPResponse) (string, error) {
	t.Helper()
	sender := &fakeSender{respond: func(int, declarative.HTTPRequest) (declarative.HTTPResponse, error) { return resp, nil }}
	n := newNode(t, sender, &fakeBuckets{}, newFakeJournal())
	out, err := n.Execute(t.Context(), callOf(httpNode(t, params), 1))
	if err != nil {
		return "", err
	}
	items := out.Ports[0].Batch.Items()
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	return marshalValue(t, items[0].JSON), nil
}

func TestResponseFormats(t *testing.T) {
	const html = "<html><body>Example Domain</body></html>"
	b64 := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x00\x01"))
	tests := []struct {
		name   string
		extra  string
		resp   declarative.HTTPResponse
		want   string
		wantIn string
	}{
		{"autodetect html is text", "", reply(200, "text/html; charset=UTF-8", html), `{"data":"` + html + `"}`, ""},
		{"autodetect plain text", "", reply(200, "text/plain", "hello"), `{"data":"hello"}`, ""},
		{"autodetect xml is text", "", reply(200, "application/xml", "<a/>"), `{"data":"<a/>"}`, ""},
		{"autodetect json is parsed", "", reply(200, "application/json", `{"id":7}`), `{"id":7}`, ""},
		{"autodetect vendor json is parsed", "", reply(200, "application/vnd.api+json", `{"id":8}`), `{"id":8}`, ""},
		{"autodetect json mislabelled falls back to text", "", reply(200, "application/json", "not json"), `{"data":"not json"}`, ""},
		{"autodetect without content type, json", "", reply(200, "", `{"id":9}`), `{"id":9}`, ""},
		{"autodetect without content type, text", "", reply(200, "", "plain words"), `{"data":"plain words"}`, ""},
		{"autodetect binary is a file", "", reply(200, "image/png", "\x89PNG\r\n\x00\x01"),
			`{"binary":{"data":{"data":"` + b64 + `","mimeType":"image/png","fileSize":8}}}`, ""},
		{"autodetect octet-stream is a file", "", reply(200, "application/octet-stream", "ab"),
			`{"binary":{"data":{"data":"YWI=","mimeType":"application/octet-stream","fileSize":2}}}`, ""},
		{"explicit text keeps json as text", `,"responseFormat":"text"`, reply(200, "application/json", `{"id":7}`), `{"data":"{\"id\":7}"}`, ""},
		{"explicit file keeps text as a file", `,"responseFormat":"file"`, reply(200, "text/plain", "hi"),
			`{"binary":{"data":{"data":"aGk=","mimeType":"text/plain","fileSize":2}}}`, ""},
		{"file name from content disposition", `,"responseFormat":"file"`, declarative.HTTPResponse{Status: 200, Header: map[string][]string{
			"content-type": {"application/pdf"}, "content-disposition": {`attachment; filename="r.pdf"`}}, Body: []byte("x")},
			`{"binary":{"data":{"data":"eA==","mimeType":"application/pdf","fileSize":1,"fileName":"r.pdf"}}}`, ""},
		{"explicit json parses", `,"responseFormat":"json"`, reply(200, "text/html", `{"id":1}`), `{"id":1}`, ""},
		{"empty body is null", "", reply(204, "text/html", ""), `null`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runGet(t, `{"url":"https://api.example.com/x"`+tt.extra+`}`, tt.resp)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if got != tt.want {
				t.Fatalf("item\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestExplicitJSONStillFailsOnAnotherBody(t *testing.T) {
	_, err := runGet(t, `{"url":"https://api.example.com/x","responseFormat":"json"}`, reply(200, "text/html", "<html>"))
	if err == nil || !strings.Contains(err.Error(), "ProviderError") {
		t.Fatalf("err = %v, want the permanent ProviderError of a body that is not JSON", err)
	}
}

func TestUnknownResponseFormatIsRefused(t *testing.T) {
	_, err := runGet(t, `{"url":"https://api.example.com/x","responseFormat":"xml"}`, reply(200, "text/html", "x"))
	if err == nil || !strings.Contains(err.Error(), "responseFormat") {
		t.Fatalf("err = %v, want a refusal of the format", err)
	}
}

func TestTextBodyIsCutAtTheInlineCap(t *testing.T) {
	// "é" has two bytes: the cut must not split it.
	body := strings.Repeat("é", MaxInlineBodyBytes/2+10)
	got, err := runGet(t, `{"url":"https://api.example.com/x"}`, reply(200, "text/plain", body))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(got, `"truncated":true`) || !strings.Contains(got, `"size":`) {
		t.Fatalf("a cut text does not say so: %.80s", got)
	}
	if strings.Contains(got, "�") {
		t.Fatal("the cut split a character")
	}
	if len(got) > MaxInlineBodyBytes+200 {
		t.Fatalf("the item is %d bytes, beyond the cap", len(got))
	}
}

func TestBinaryBodyBeyondTheCapFailsPermanently(t *testing.T) {
	body := strings.Repeat("\x00", MaxInlineBodyBytes+1)
	_, err := runGet(t, `{"url":"https://api.example.com/x"}`, reply(200, "application/zip", body))
	if err == nil || !strings.Contains(err.Error(), "PERMANENT PayloadTooLarge") {
		t.Fatalf("err = %v, want a permanent PayloadTooLarge", err)
	}
	if errors.Is(err, ErrUnknownOutcome) {
		t.Fatal("a refused body is not an unknown outcome")
	}
}

// TestSecretsAreMaskedInTextAndFileBodies keeps the masking of ADR-0043 section 5 for the new shapes: a
// host that echoes the credential in an HTML page, or in a file, must not put it into the item.
func TestSecretsAreMaskedInTextAndFileBodies(t *testing.T) {
	const secret = "SUPERSECRET-token-123"
	for _, tc := range []struct {
		name, format, contentType string
	}{
		{"text", "autodetect", "text/html"},
		{"file", "file", "application/octet-stream"},
		{"text forced over json", "text", "application/json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sender := &fakeSender{respond: func(int, declarative.HTTPRequest) (declarative.HTTPResponse, error) {
				return reply(200, tc.contentType, "<p>you sent "+secret+"</p>"), nil
			}}
			n, err := New(Config{
				Journal: newFakeJournal(), Scopes: testScopes{}, Buckets: &fakeBuckets{},
				SenderFor: func(*url.URL) declarative.Sender { return sender },
				Credential: func(context.Context, CredentialRequest) (Credential, error) {
					return Credential{Auth: declarative.Auth{Type: declarative.AuthBearer, Field: "token"},
						Fields: map[string]string{"token": secret}, Mask: []string{secret}}, nil
				},
				Bucket: limit.Spec{Capacity: 100, RefillPerSecond: 50},
				IDs:    id.NewV7Generator(func() time.Time { return t0 }), Clock: clock.NewFake(t0),
			})
			if err != nil {
				t.Fatal(err)
			}
			out, err := n.Execute(t.Context(), callOf(httpNode(t, `{"url":"https://api.example.com/x","credential":"c","responseFormat":"`+tc.format+`"}`), 1))
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			got := marshalValue(t, out.Ports[0].Batch.Items()[0].JSON)
			if strings.Contains(got, secret) {
				t.Fatalf("the secret is in the item: %s", got)
			}
			if tc.format == "file" {
				_, afterKey, _ := strings.Cut(got, `"data":"`)
				raw, _, _ := strings.Cut(afterKey, `"`)
				dec, err := base64.StdEncoding.DecodeString(raw)
				if err != nil || strings.Contains(string(dec), secret) || !strings.Contains(string(dec), "[REDACTED]") {
					t.Fatalf("the file bytes are not masked: %q, %v", dec, err)
				}
			} else if !strings.Contains(got, "[REDACTED]") {
				t.Fatalf("nothing was redacted: %s", got)
			}
		})
	}
}
