// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/wirvii/orcon/internal/adapter/codec/declarativejson"
	"github.com/wirvii/orcon/internal/adapter/connectors/whatsapp"
	"github.com/wirvii/orcon/internal/connector/declarative"
	httpdispatch "github.com/wirvii/orcon/internal/connector/dispatch"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/trigger"
)

// whatsappTriggerType is the node type of the inbound WhatsApp trigger: the
// connector id, then the trigger name the connector's connector.json declares.
const whatsappTriggerType = "orcon.whatsapp.whatsappTrigger@1"

// whatsappTriggerName is the key of the trigger in the connector body.
const whatsappTriggerName = "whatsappTrigger"

// whatsappCredential names the credential of the WhatsApp connector. Its field
// appSecret checks the signature of every inbound request, its field
// verifyToken answers the subscription handshake, and its field token is the
// bearer token of the outgoing messages.
const whatsappCredential = "whatsapp"

// credentialResolver returns the credential a node or trigger names, for one
// organization. The credential store implements it; until it exists the
// composition root refuses every name.
type credentialResolver func(ctx context.Context, org identity.OrgID, credentialID string) (httpdispatch.Credential, error)

// whatsappVerifier authenticates the public requests of the routes that start
// flows with the WhatsApp trigger. A route of any other node type is not
// authenticated here: its request is taken as it arrives, as it was before this
// verifier existed.
type whatsappVerifier struct {
	trigger     declarative.Trigger
	flows       trigger.FlowSource
	credentials credentialResolver
}

var _ trigger.Verifier = whatsappVerifier{}

// newWhatsAppVerifier reads the declared WhatsApp trigger from the connector
// package that ships with this build and checks that it is runnable.
func newWhatsAppVerifier(flows trigger.FlowSource, credentials credentialResolver) (whatsappVerifier, error) {
	data, err := fs.ReadFile(whatsapp.Files, "connector.json")
	if err != nil {
		return whatsappVerifier{}, fmt.Errorf("read the WhatsApp connector: %w", err)
	}
	def, err := declarativejson.Decode(data)
	if err != nil {
		return whatsappVerifier{}, fmt.Errorf("decode the WhatsApp connector: %w", err)
	}
	if err := declarative.Validate(def); err != nil {
		return whatsappVerifier{}, fmt.Errorf("the WhatsApp connector is not runnable: %w", err)
	}
	t, ok := def.Triggers[whatsappTriggerName]
	if !ok || t.Verify == nil || t.Handshake == nil || t.Decode == nil {
		return whatsappVerifier{}, errors.New("the WhatsApp connector declares no complete inbound trigger")
	}
	return whatsappVerifier{trigger: t, flows: flows, credentials: credentials}, nil
}

// Admit implements [trigger.Verifier]. For a WhatsApp route it checks the
// signature with the app secret, then turns the body into one item per message:
// a notification that carries no message (a delivery receipt, for example)
// starts nothing.
func (v whatsappVerifier) Admit(ctx context.Context, r trigger.Resolved, req trigger.IngressRequest) ([]byte, bool, error) {
	nodeType, err := v.nodeType(ctx, r)
	if err != nil {
		return nil, false, err
	}
	if nodeType != whatsappTriggerType {
		return req.Body, true, nil
	}
	cred, err := v.credentials(ctx, r.Org, whatsappCredential)
	if err != nil {
		return nil, false, err
	}
	secret := cred.Fields[v.trigger.Verify.Field]
	if secret == "" {
		return nil, false, fmt.Errorf("credential %q has no %s field", whatsappCredential, v.trigger.Verify.Field)
	}
	if err := declarative.VerifySignature(*v.trigger.Verify, req.Headers, req.Body, secret); err != nil {
		return nil, false, err
	}
	body, err := jsonvalue.Parse(req.Body)
	if err != nil {
		return nil, false, fmt.Errorf("the body is not JSON: %w", err)
	}
	events, err := v.trigger.Decode.Events(body)
	if err != nil {
		return nil, false, fmt.Errorf("decode the WhatsApp events: %w", err)
	}
	if len(events) == 0 {
		return nil, false, nil
	}
	out, err := jsonvalue.Marshal(jsonvalue.Array(events...))
	if err != nil {
		return nil, false, fmt.Errorf("encode the WhatsApp events: %w", err)
	}
	return out, true, nil
}

// Challenge implements [trigger.Verifier]: it answers the subscription handshake
// of a WhatsApp route, and refuses every other route, which declares none.
func (v whatsappVerifier) Challenge(ctx context.Context, r trigger.Resolved, req trigger.IngressRequest) (string, error) {
	nodeType, err := v.nodeType(ctx, r)
	if err != nil {
		return "", err
	}
	if nodeType != whatsappTriggerType {
		return "", errors.New("the trigger declares no handshake")
	}
	cred, err := v.credentials(ctx, r.Org, whatsappCredential)
	if err != nil {
		return "", err
	}
	return declarative.CheckHandshake(*v.trigger.Handshake, req.Query, cred.Fields[v.trigger.Handshake.Field])
}

// nodeType reads the node type of the route from the active version of its flow.
func (v whatsappVerifier) nodeType(ctx context.Context, r trigger.Resolved) (string, error) {
	rec, err := v.flows.GetFlow(ctx, r.Org, r.FlowID)
	if err != nil {
		return "", fmt.Errorf("read the flow of the route: %w", err)
	}
	ver, err := v.flows.GetVersion(ctx, r.Org, rec.ActiveVersionID)
	if err != nil {
		return "", fmt.Errorf("read the live version of the route: %w", err)
	}
	doc, err := (flowReader{}).Read(ver.Document)
	if err != nil {
		return "", err
	}
	for i := range doc.Nodes {
		if doc.Nodes[i].ID == r.NodeID {
			return doc.Nodes[i].Type, nil
		}
	}
	return "", fmt.Errorf("the live version has no node %s", r.NodeID)
}

// triggerCatalog declares the trigger node types of this build: the native
// webhook and the WhatsApp inbound trigger. It answers for no other type.
type triggerCatalog struct{}

// LookupTrigger implements [trigger.Catalog].
func (triggerCatalog) LookupTrigger(_ context.Context, nodeType string) (trigger.Declaration, bool) {
	switch nodeType {
	case WebhookNodeType, whatsappTriggerType:
		return trigger.Declaration{
			Family: flow.TriggerWebhook, Delivery: trigger.DeliveryAtLeastOnce, MaxPayloadBytes: webhookBodyCap,
		}, true
	default:
		return trigger.Declaration{}, false
	}
}
