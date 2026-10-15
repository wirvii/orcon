// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package whatsapp_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/test/mockprovider"
)

// itemText is the canonical JSON text of one emitted item.
func itemText(t *testing.T, item jsonvalue.Value) string {
	t.Helper()
	text, err := jsonvalue.Canonical(item)
	if err != nil {
		t.Fatalf("item is not encodable: %v", err)
	}
	return string(text)
}

// fixedSender answers every request with one recorded reply of the Graph API.
// It stands in for the network: no socket is opened.
type fixedSender struct {
	status int
	body   string
}

func (f fixedSender) Send(context.Context, declarative.HTTPRequest) (declarative.HTTPResponse, error) {
	return declarative.HTTPResponse{
		Status: f.status,
		Header: map[string][]string{"content-type": {"application/json"}},
		Body:   []byte(f.body),
	}, nil
}

// metaErrors are error replies in the shape the Graph API sends them: an
// "error" object with the numeric code, the subcode and the trace id.
var metaErrors = []struct {
	name   string
	status int
	body   string
	class  declarative.Class
	code   string
}{
	{
		name: "24-hour window closed", status: 400,
		body:  `{"error":{"message":"(#131047) Re-engagement message","type":"OAuthException","code":131047,"error_data":{"messaging_product":"whatsapp","details":"More than 24 hours have passed since the customer last replied to this number."},"error_subcode":2494055,"fbtrace_id":"AbCd1234Ef5"}}`,
		class: declarative.ClassPermanent, code: "ValidationFailed",
	},
	{
		name: "recipient not on WhatsApp", status: 400,
		body:  `{"error":{"message":"(#131026) Message undeliverable","type":"OAuthException","code":131026,"error_subcode":2593109,"fbtrace_id":"Zy98Xw76"}}`,
		class: declarative.ClassPermanent, code: "ValidationFailed",
	},
	{
		name: "throughput level reached", status: 400,
		body:  `{"error":{"message":"(#130429) Throughput level is reached","type":"OAuthException","code":130429,"error_subcode":2494055,"is_transient":true,"fbtrace_id":"Qw12Er34"}}`,
		class: declarative.ClassTransient, code: "RateLimited",
	},
	{
		name: "WhatsApp Business Account rate limit", status: 400,
		body:  `{"error":{"message":"(#80007) Rate limit reached for the WhatsApp Business Account","type":"OAuthException","code":80007,"fbtrace_id":"Mn45Op56"}}`,
		class: declarative.ClassTransient, code: "RateLimited",
	},
	{
		name: "pair rate limit", status: 400,
		body:  `{"error":{"message":"(#131056) Pair rate limit hit","type":"OAuthException","code":131056,"error_subcode":2494055,"fbtrace_id":"Lk78Jh90"}}`,
		class: declarative.ClassTransient, code: "RateLimited",
	},
	{
		name: "expired access token", status: 401,
		body:  `{"error":{"message":"Error validating access token: Session has expired","type":"OAuthException","code":190,"error_subcode":463,"fbtrace_id":"Ab12Cd34"}}`,
		class: declarative.ClassPermanent, code: "AuthInvalid",
	},
	{
		name: "missing permission", status: 403,
		body:  `{"error":{"message":"(#10) Application does not have permission","type":"OAuthException","code":10,"fbtrace_id":"Ef56Gh78"}}`,
		class: declarative.ClassPermanent, code: "PermissionDenied",
	},
}

// TestMetaErrorsClassifyByBodyCode: a failed send is classified by the Graph
// error code when the status alone is not enough. Throughput codes sent with
// HTTP 400 must not be reported as validation failures.
func TestMetaErrorsClassifyByBodyCode(t *testing.T) {
	def := loadDefinition(t)
	for _, c := range metaErrors {
		t.Run(c.name, func(t *testing.T) {
			in, err := declarative.New(def, declarative.Options{
				Sender: fixedSender{status: c.status, body: c.body},
				Sleep:  noSleep,
			})
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}
			_, callErr := in.Execute(t.Context(), declarative.Call{
				Operation:   "message.send",
				Parameters:  params(t, map[string]string{"phoneNumberId": phoneNumberID, "to": "573000000000", "text": `{"body":"Hola"}`}),
				Credentials: map[string]string{"token": mockprovider.Token},
			}, nil)
			var ce *declarative.CallError
			if !errors.As(callErr, &ce) {
				t.Fatalf("want failure %s/%s, got %v", c.class, c.code, callErr)
			}
			if ce.Class != c.class || ce.Code != c.code {
				t.Errorf("failure is %s/%s, want %s/%s", ce.Class, ce.Code, c.class, c.code)
			}
		})
	}
}

// TestMetaSendReplyKeepsTheRecordAndAddsIdAndRecipient: the provider's reply
// stays as it is, and the top-level message id and the resolved recipient are
// added to it (NM-10).
func TestMetaSendReplyMapsToIdAndRecipient(t *testing.T) {
	def := loadDefinition(t)
	const reply = `{"messaging_product":"whatsapp","contacts":[{"input":"+573000000000","wa_id":"573000000000"}],"messages":[{"id":"wamid.HBgMNTczMDAwMDAwMDAwMBUCABIYIDNBMzk1RkU="}]}`
	in, err := declarative.New(def, declarative.Options{
		Sender: fixedSender{status: http.StatusOK, body: reply},
		Sleep:  noSleep,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	res, err := in.Execute(t.Context(), declarative.Call{
		Operation:   "message.send",
		Parameters:  params(t, map[string]string{"phoneNumberId": phoneNumberID, "to": "573000000000", "text": `{"body":"Hola"}`}),
		Credentials: map[string]string{"token": mockprovider.Token},
	}, nil)
	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}
	const want = `{"contacts":[{"input":"+573000000000","wa_id":"573000000000"}],"id":"wamid.HBgMNTczMDAwMDAwMDAwMBUCABIYIDNBMzk1RkU=","messages":[{"id":"wamid.HBgMNTczMDAwMDAwMDAwMBUCABIYIDNBMzk1RkU="}],"messaging_product":"whatsapp","recipient":"573000000000"}`
	if len(res.Items) != 1 || canonical(t, want) != canonical(t, itemText(t, res.Items[0])) {
		t.Fatalf("items = %v, want one item %s", res.Items, want)
	}
}
