// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package whatsapp_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/connectors/whatsapp"
	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/test/mockprovider"
)

// phoneNumberID is the Graph node the mock provider serves messages for.
const phoneNumberID = mockprovider.MockPhoneNumberID

// packageFiles returns the files of the embedded package, as the package
// builder reads them.
func packageFiles(t *testing.T) []connector.PackageFile {
	t.Helper()
	var files []connector.PackageFile
	err := fs.WalkDir(whatsapp.Files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(whatsapp.Files, path)
		if err != nil {
			return err
		}
		files = append(files, connector.PackageFile{Path: path, Data: data})
		return nil
	})
	if err != nil {
		t.Fatalf("reading the embedded package: %v", err)
	}
	return files
}

// loadDefinition decodes and validates the declarative body of the package.
func loadDefinition(t *testing.T) declarative.Definition {
	t.Helper()
	data, err := fs.ReadFile(whatsapp.Files, "connector.json")
	if err != nil {
		t.Fatalf("reading connector.json: %v", err)
	}
	def := decodeBody(t, data)
	if err := declarative.Validate(def); err != nil {
		t.Fatalf("connector.json is not runnable: %v", err)
	}
	return def
}

// object is one JSON object of the body, read with the member names it allows.
// Any other member fails the test, so a misspelled key cannot silently turn a
// rule off, which is the same strictness as the production codec.
type object struct {
	t       *testing.T
	path    string
	members map[string]jsonvalue.Value
}

func objectOf(t *testing.T, v jsonvalue.Value, path string, allowed ...string) object {
	t.Helper()
	if v.Kind() != jsonvalue.KindObject {
		t.Fatalf("%s must be an object", path)
	}
	o := object{t: t, path: path, members: map[string]jsonvalue.Value{}}
	for _, m := range v.Members() {
		if !slices.Contains(allowed, m.Key) {
			t.Fatalf("%s: unknown member %q", path, m.Key)
		}
		o.members[m.Key] = m.Value
	}
	return o
}

func (o object) value(key string) (jsonvalue.Value, bool) {
	v, ok := o.members[key]
	return v, ok
}

func (o object) str(key string) string {
	v, ok := o.value(key)
	if !ok {
		return ""
	}
	s, isText := v.Str()
	if !isText {
		o.t.Fatalf("%s/%s must be a string", o.path, key)
	}
	return s
}

// nodeMap reads an object whose members are body templates of any JSON type.
func (o object) nodeMap(key string) map[string]declarative.Node {
	out := map[string]declarative.Node{}
	if v, ok := o.value(key); ok {
		for _, m := range v.Members() {
			out[m.Key] = declarative.Node{Value: m.Value}
		}
	}
	return out
}

func (o object) child(key string, allowed ...string) object {
	v, ok := o.value(key)
	if !ok {
		return object{t: o.t, path: o.path + "/" + key, members: map[string]jsonvalue.Value{}}
	}
	return objectOf(o.t, v, o.path+"/"+key, allowed...)
}

func (o object) strMap(key string) map[string]string {
	out := map[string]string{}
	if v, ok := o.value(key); ok {
		for _, m := range v.Members() {
			s, isText := m.Value.Str()
			if !isText {
				o.t.Fatalf("%s/%s/%s must be a string", o.path, key, m.Key)
			}
			out[m.Key] = s
		}
	}
	return out
}

// decodeBody reads connector.json into the interpreter's types. It mirrors the
// production codec field by field: depguard keeps encoding/json and the codec
// away from connector packages, so the test reads the document through
// jsonvalue instead.
func decodeBody(t *testing.T, data []byte) declarative.Definition {
	t.Helper()
	root, err := jsonvalue.Parse(data)
	if err != nil {
		t.Fatalf("connector.json is not JSON: %v", err)
	}
	top := objectOf(t, root, "connector.json", "format", "http", "operations", "triggers")
	def := declarative.Definition{Format: top.str("format"), Operations: map[string]declarative.Operation{}}

	h := top.child("http", "baseUrl", "headers", "timeout", "auth")
	def.HTTP = declarative.HTTP{BaseURL: h.str("baseUrl"), Headers: h.strMap("headers"), Timeout: h.str("timeout")}
	a := h.child("auth", "type", "in", "name", "prefix", "field", "headers", "service", "region")
	def.HTTP.Auth = declarative.Auth{Type: a.str("type"), In: a.str("in"), Name: a.str("name"), Prefix: a.str("prefix"), Field: a.str("field"), Headers: a.strMap("headers"), Service: a.str("service"), Region: a.str("region")}

	ops, _ := top.value("operations")
	for _, m := range ops.Members() {
		o := objectOf(t, m.Value, "operations/"+m.Key, "purity", "safety", "request", "response")
		op := declarative.Operation{Purity: o.str("purity"), Safety: o.str("safety")}
		req := o.child("request", "method", "path", "query", "headers", "body")
		op.Request = declarative.Request{Method: req.str("method"), Path: req.str("path"), Query: req.strMap("query"), Headers: req.strMap("headers")}
		if body := req.child("body", "type", "fields"); len(body.members) > 0 {
			op.Request.Body = &declarative.Body{Type: body.str("type"), Fields: body.nodeMap("fields")}
		}
		resp := o.child("response", "select", "map", "empty", "errors")
		op.Response = declarative.ResponseSpec{Select: resp.str("select"), Map: resp.strMap("map"), Empty: resp.str("empty")}
		if errs, ok := resp.value("errors"); ok {
			for i := range errs.Len() {
				item, _ := errs.At(i)
				r := objectOf(t, item, fmt.Sprintf("errors/%d", i), "status", "body", "class", "code")
				rule := declarative.ErrorRule{Class: r.str("class"), Code: r.str("code")}
				if statuses, ok := r.value("status"); ok {
					for j := range statuses.Len() {
						s, _ := statuses.At(j)
						n, ok := s.Int64()
						if !ok {
							t.Fatalf("errors/%d/status must be integers", i)
						}
						rule.Status = append(rule.Status, int(n))
					}
				}
				if b, ok := r.value("body"); ok {
					bm := objectOf(t, b, fmt.Sprintf("errors/%d/body", i), "path", "in")
					match := &declarative.BodyMatch{Path: bm.str("path")}
					in, _ := bm.value("in")
					for j := range in.Len() {
						s, _ := in.At(j)
						match.In = append(match.In, declarative.Node{Value: s})
					}
					rule.Body = match
				}
				op.Response.Errors = append(op.Response.Errors, rule)
			}
		}
		def.Operations[m.Key] = op
	}
	if tv, ok := top.value("triggers"); ok {
		def.Triggers = map[string]declarative.Trigger{}
		for _, m := range tv.Members() {
			o := objectOf(t, m.Value, "triggers/"+m.Key, "family", "verify", "handshake")
			tr := declarative.Trigger{Family: o.str("family")}
			if v := o.child("verify", "type", "header", "prefix", "field"); len(v.members) > 0 {
				tr.Verify = &declarative.Verify{Type: v.str("type"), Header: v.str("header"), Prefix: v.str("prefix"), Field: v.str("field")}
			}
			if h := o.child("handshake", "type", "modeParam", "tokenParam", "challengeParam", "field"); len(h.members) > 0 {
				tr.Handshake = &declarative.Handshake{
					Type: h.str("type"), ModeParam: h.str("modeParam"), TokenParam: h.str("tokenParam"),
					ChallengeParam: h.str("challengeParam"), Field: h.str("field"),
				}
			}
			def.Triggers[m.Key] = tr
		}
	}
	return def
}

// graphSender sends the requests of the interpreter to the mock provider in
// process: the handler is the provider, no socket is opened. fault is sent in
// the X-Mock-Fault header of every request; "hangup" makes the provider apply
// the write and then reports a connection cut, which is the case the
// interpreter must treat as ambiguous.
type graphSender struct {
	handler http.Handler
	fault   string
}

func (s graphSender) Send(ctx context.Context, req declarative.HTTPRequest) (declarative.HTTPResponse, error) {
	hr, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return declarative.HTTPResponse{}, err
	}
	for k, values := range req.Header {
		for _, v := range values {
			hr.Header.Add(k, v)
		}
	}
	if s.fault != "" {
		hr.Header.Set("X-Mock-Fault", s.fault)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, hr)
	if s.fault == "hangup" {
		return declarative.HTTPResponse{}, errors.New("connection reset by peer")
	}
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return declarative.HTTPResponse{}, err
	}
	header := make(map[string][]string, len(res.Header))
	for k, values := range res.Header {
		header[strings.ToLower(k)] = values
	}
	return declarative.HTTPResponse{Status: res.StatusCode, Header: header, Body: body}, nil
}

// noSleep replaces the retry wait so the tests do not sleep.
func noSleep(context.Context, time.Duration) error { return nil }

// param reads one parameter: text that starts with "{" is a JSON object,
// anything else is a string.
func param(t *testing.T, raw string) jsonvalue.Value {
	t.Helper()
	if strings.HasPrefix(raw, "{") {
		v, err := jsonvalue.Parse([]byte(raw))
		if err != nil {
			t.Fatalf("parameter %s is not JSON: %v", raw, err)
		}
		return v
	}
	return jsonvalue.String(raw)
}

func params(t *testing.T, raw map[string]string) map[string]jsonvalue.Value {
	t.Helper()
	out := make(map[string]jsonvalue.Value, len(raw))
	for k, v := range raw {
		out[k] = param(t, v)
	}
	return out
}

// canonical returns the canonical JSON text of a JSON document.
func canonical(t *testing.T, text string) string {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(text))
	if err != nil {
		t.Fatalf("not JSON: %s: %v", text, err)
	}
	out, err := jsonvalue.Canonical(v)
	if err != nil {
		t.Fatalf("not encodable: %v", err)
	}
	return string(out)
}

func TestPackageIsValid(t *testing.T) {
	pkg, err := connector.BuildPackage(packageFiles(t), connector.Options{Official: true})
	if err != nil {
		t.Fatalf("BuildPackage() failed: %v", err)
	}
	m := pkg.Manifest()
	if m.ID != "orcon.whatsapp" || m.Kind != connector.KindDeclarative || m.Version != "1.0.0" {
		t.Fatalf("manifest identity = %s %s %s", m.ID, m.Kind, m.Version)
	}
	if _, err := connector.OpenPackage(pkg.Archive(), connector.Options{Official: true}); err != nil {
		t.Fatalf("OpenPackage() of the built archive failed: %v", err)
	}
}

// TestManifestMatchesBody: every operation of the manifest exists in the
// declarative body with the same purity and safety level, and the other way
// round. The two documents are read by different tools, so they must agree.
func TestManifestMatchesBody(t *testing.T) {
	pkg, err := connector.BuildPackage(packageFiles(t), connector.Options{Official: true})
	if err != nil {
		t.Fatalf("BuildPackage() failed: %v", err)
	}
	m := pkg.Manifest()
	def := loadDefinition(t)
	if len(m.Nodes) != 1 {
		t.Fatalf("want one action node, got %d", len(m.Nodes))
	}
	node := m.Nodes[0]
	if len(node.Operations) != len(def.Operations) {
		t.Fatalf("manifest declares %d operations, the body %d", len(node.Operations), len(def.Operations))
	}
	for _, op := range node.Operations {
		body, ok := def.Operations[op.Name]
		if !ok {
			t.Errorf("operation %s is in the manifest but not in connector.json", op.Name)
			continue
		}
		if string(op.Purity) != body.Purity {
			t.Errorf("%s: purity %s in the manifest, %s in the body", op.Name, op.Purity, body.Purity)
		}
		if op.Safety == nil {
			if body.Purity == declarative.PurityEffect {
				t.Errorf("%s: an EFFECT operation needs a safety contract", op.Name)
			}
			continue
		}
		if string(op.Safety.Level) != body.Safety {
			t.Errorf("%s: safety %s in the manifest, %s in the body", op.Name, op.Safety.Level, body.Safety)
		}
	}
	if got := string(m.Credentials[0].Scheme.Type); got != def.HTTP.Auth.Type {
		t.Errorf("credential scheme %s in the manifest, auth %s in the body", got, def.HTTP.Auth.Type)
	}
}

// callCase is one recorded exchange with the mock provider: the call, the
// fault it provokes, and what must be observed afterwards.
type callCase struct {
	name      string
	operation string
	params    map[string]string
	token     string
	fault     string
	// wantMethod and wantPath are checked on the first request.
	wantMethod string
	wantPath   string
	// wantBody is the canonical body of the request, when the operation has one.
	wantBody string
	// wantItems is the number of items of a successful call. wantItem, when set,
	// is the canonical first item.
	wantItems int
	wantItem  string
	// wantClass and wantCode are the failure of the call; empty means success.
	wantClass declarative.Class
	wantCode  string
	// wantSent is how many requests reached the provider; wantApplied how many
	// writes it applied. A write sent once and never resent is sent == 1.
	wantSent    int
	wantApplied int
}

const (
	textBody     = `{"messaging_product":"whatsapp","recipient_type":"individual","to":"573001234567","type":"text","text":{"body":"Hola"}}`
	templateBody = `{"messaging_product":"whatsapp","recipient_type":"individual","to":"573001234567","type":"template","template":{"name":"order_update","language":{"code":"es_CO"}}}`
	imageBody    = `{"messaging_product":"whatsapp","recipient_type":"individual","to":"573001234567","type":"image","image":{"link":"https://cdn.example.com/a.jpg","caption":"Hola"}}`
	documentBody = `{"messaging_product":"whatsapp","recipient_type":"individual","to":"573001234567","type":"document","document":{"link":"https://cdn.example.com/a.pdf","filename":"cotizacion.pdf"}}`
	readBody     = `{"messaging_product":"whatsapp","status":"read","message_id":"wamid.HBgLMTIzNDU2"}`
	messagesPath = "/v25.0/" + phoneNumberID + "/messages"
	// The sentItem constant is the output of a send: the message id and the
	// recipient the provider resolved, as the mock provider answers them.
	sentItem = `{"contacts":[{"input":"` + mockprovider.MockContact + `","wa_id":"` + mockprovider.MockContact + `"}],` +
		`"id":"wamid.mock-1","messages":[{"id":"wamid.mock-1"}],"messaging_product":"whatsapp",` +
		`"recipient":"` + mockprovider.MockContact + `"}`
)

var callCases = []callCase{
	{
		name: "text message is posted to the messages endpoint", operation: "message.send",
		params:     map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567", "text": `{"body":"Hola"}`},
		wantMethod: "POST", wantPath: messagesPath, wantBody: textBody, wantItems: 1, wantItem: sentItem, wantSent: 1, wantApplied: 1,
	},
	{
		name: "template message carries the template object", operation: "templateMessage.send",
		params:     map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567", "template": `{"name":"order_update","language":{"code":"es_CO"}}`},
		wantMethod: "POST", wantPath: messagesPath, wantBody: templateBody, wantItems: 1, wantSent: 1, wantApplied: 1,
	},
	{
		name: "image by link", operation: "imageMessage.send",
		params:     map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567", "image": `{"link":"https://cdn.example.com/a.jpg","caption":"Hola"}`},
		wantMethod: "POST", wantPath: messagesPath, wantBody: imageBody, wantItems: 1, wantSent: 1, wantApplied: 1,
	},
	{
		name: "document by link", operation: "documentMessage.send",
		params:     map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567", "document": `{"link":"https://cdn.example.com/a.pdf","filename":"cotizacion.pdf"}`},
		wantMethod: "POST", wantPath: messagesPath, wantBody: documentBody, wantItems: 1, wantSent: 1, wantApplied: 1,
	},
	{
		name: "mark as read posts the status", operation: "message.update",
		params:     map[string]string{"phoneNumberId": phoneNumberID, "messageId": "wamid.HBgLMTIzNDU2"},
		wantMethod: "POST", wantPath: messagesPath, wantBody: readBody, wantItems: 1, wantSent: 1, wantApplied: 1,
	},
	{
		name: "phone number test reads the phone number", operation: "phoneNumber.get",
		params:     map[string]string{"phoneNumberId": phoneNumberID},
		wantMethod: "GET", wantPath: "/v25.0/" + phoneNumberID, wantItems: 1, wantSent: 1,
	},
	{
		name: "a wrong token is refused as AuthInvalid", operation: "message.send",
		params:     map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567", "text": `{"body":"Hola"}`},
		token:      "wrong-token",
		wantMethod: "POST", wantPath: messagesPath, wantClass: declarative.ClassPermanent, wantCode: "AuthInvalid", wantSent: 1,
	},
	{
		name: "a rate limit on a send is transient and never resent", operation: "message.send",
		params:     map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567", "text": `{"body":"Hola"}`},
		fault:      "429",
		wantMethod: "POST", wantPath: messagesPath, wantClass: declarative.ClassTransient, wantCode: "RateLimited", wantSent: 1,
	},
	{
		name: "a validation error on a send is permanent", operation: "message.send",
		params:     map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567", "text": `{"body":"Hola"}`},
		fault:      "400",
		wantMethod: "POST", wantPath: messagesPath, wantClass: declarative.ClassPermanent, wantCode: "ValidationFailed", wantSent: 1,
	},
	{
		name: "a throughput limit carried in the body is transient, not a validation error", operation: "message.send",
		params:     map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567", "text": `{"body":"Hola"}`},
		fault:      "throttle",
		wantMethod: "POST", wantPath: messagesPath, wantClass: declarative.ClassTransient, wantCode: "RateLimited", wantSent: 1,
	},
	{
		name: "a server error after a send is uncertain and never resent", operation: "message.send",
		params:     map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567", "text": `{"body":"Hola"}`},
		fault:      "5xx",
		wantMethod: "POST", wantPath: messagesPath, wantClass: declarative.ClassUncertain, wantCode: "ServiceUnavailable", wantSent: 1, wantApplied: 1,
	},
	{
		name: "a connection cut after a send is uncertain and never resent", operation: "message.send",
		params:     map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567", "text": `{"body":"Hola"}`},
		fault:      "hangup",
		wantMethod: "POST", wantPath: messagesPath, wantClass: declarative.ClassUncertain, wantCode: "NetworkError", wantSent: 1, wantApplied: 1,
	},
	{
		name: "a read is retried on a rate limit up to the default attempts", operation: "phoneNumber.get",
		params:     map[string]string{"phoneNumberId": phoneNumberID},
		fault:      "429",
		wantMethod: "GET", wantPath: "/v25.0/" + phoneNumberID, wantClass: declarative.ClassTransient, wantCode: "RateLimited", wantSent: declarative.DefaultMaxAttempts,
	},
	{
		name: "a missing parameter fails before anything is sent", operation: "message.send",
		params:    map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567"},
		wantClass: declarative.ClassPermanent, wantCode: "ValidationFailed", wantSent: 0,
	},
}

// TestOperationsAgainstMockProvider runs every case against the mock provider,
// in process, and checks the request that reached it, the items or the failure
// the caller sees, and the number of writes the provider applied.
func TestOperationsAgainstMockProvider(t *testing.T) {
	def := loadDefinition(t)
	for _, c := range callCases {
		t.Run(c.name, func(t *testing.T) {
			provider := mockprovider.New()
			token := c.token
			if token == "" {
				token = mockprovider.Token
			}
			in, err := declarative.New(def, declarative.Options{
				Sender: graphSender{handler: provider, fault: c.fault},
				Sleep:  noSleep,
			})
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}
			res, callErr := in.Execute(t.Context(), declarative.Call{
				Operation:   c.operation,
				Parameters:  params(t, c.params),
				Credentials: map[string]string{"token": token},
			}, nil)

			sent := provider.Requests()
			if len(sent) != c.wantSent {
				t.Errorf("provider received %d requests, want %d", len(sent), c.wantSent)
			}
			if c.wantSent > 0 && len(sent) > 0 {
				first := sent[0]
				if first.Method != c.wantMethod || first.Path != c.wantPath {
					t.Errorf("first request is %s %s, want %s %s", first.Method, first.Path, c.wantMethod, c.wantPath)
				}
				if first.Authorization != "Bearer "+token {
					t.Errorf("first request carries the wrong authorization")
				}
				if c.wantBody != "" {
					if got := canonical(t, string(first.Body)); got != canonical(t, c.wantBody) {
						t.Errorf("request body is %s, want %s", got, canonical(t, c.wantBody))
					}
				}
			}
			if got := provider.Applied(); got != c.wantApplied {
				t.Errorf("provider applied %d writes, want %d", got, c.wantApplied)
			}

			if c.wantClass != "" {
				var ce *declarative.CallError
				if !errors.As(callErr, &ce) {
					t.Fatalf("want failure %s/%s, got %v", c.wantClass, c.wantCode, callErr)
				}
				if ce.Class != c.wantClass || ce.Code != c.wantCode {
					t.Errorf("failure is %s/%s, want %s/%s", ce.Class, ce.Code, c.wantClass, c.wantCode)
				}
				return
			}
			if callErr != nil {
				t.Fatalf("unexpected failure: %v", callErr)
			}
			if len(res.Items) != c.wantItems {
				t.Fatalf("got %d items, want %d", len(res.Items), c.wantItems)
			}
			for i, item := range res.Items {
				text, err := jsonvalue.Canonical(item)
				if err != nil {
					t.Fatalf("item %d is not encodable: %v", i, err)
				}
				if i == 0 && c.wantItem != "" && string(text) != canonical(t, c.wantItem) {
					t.Errorf("first item is %s, want %s", text, canonical(t, c.wantItem))
				}
				if strings.Contains(string(text), token) {
					t.Errorf("item %d carries the credential value", i)
				}
			}
		})
	}
}

// TestSendIsNeverRetriedAfterDispatch: a write that the provider may have
// applied is never repeated by the interpreter, whatever the failure.
func TestSendIsNeverRetriedAfterDispatch(t *testing.T) {
	def := loadDefinition(t)
	for _, fault := range []string{"5xx", "hangup"} {
		provider := mockprovider.New()
		in, err := declarative.New(def, declarative.Options{Sender: graphSender{handler: provider, fault: fault}, Sleep: noSleep})
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}
		_, _ = in.Execute(t.Context(), declarative.Call{
			Operation:   "message.send",
			Parameters:  params(t, map[string]string{"phoneNumberId": phoneNumberID, "to": "573001234567", "text": `{"body":"Hola"}`}),
			Credentials: map[string]string{"token": mockprovider.Token},
		}, nil)
		if got := provider.Applied(); got != 1 {
			t.Errorf("fault %s: %d writes applied, want exactly 1", fault, got)
		}
	}
}
