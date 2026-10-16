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
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/trigger"
)

// whatsappTriggerType is the node type of the inbound WhatsApp trigger: the
// connector id, then the trigger name the connector's connector.json declares.
const whatsappTriggerType = "orcon.whatsapp.whatsappTrigger@1"

// whatsappTriggerName is the key of the trigger in the connector body.
const whatsappTriggerName = "whatsappTrigger"

// credentialParameter is the parameter of the trigger node that names the stored
// credential of the connector, by identifier, as the HTTP node names its own. Its
// field appSecret checks the signature of every inbound request and its field
// verifyToken answers the subscription handshake; the field token of the same
// credential is the bearer token of the outgoing messages.
const credentialParameter = "credential"

// whatsappVerifier authenticates the public requests of the routes that start
// flows with the WhatsApp trigger. A route of any other node type is not
// authenticated here: its request is taken as it arrives, as it was before this
// verifier existed.
type whatsappVerifier struct {
	trigger     declarative.Trigger
	flows       trigger.FlowSource
	credentials inboundFields
}

var _ trigger.Verifier = whatsappVerifier{}

// newWhatsAppVerifier reads the declared WhatsApp trigger from the connector
// package that ships with this build and checks that it is runnable.
func newWhatsAppVerifier(flows trigger.FlowSource, credentials inboundFields) (whatsappVerifier, error) {
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
	node, err := v.node(ctx, r)
	if err != nil {
		return nil, false, err
	}
	if node.Type != whatsappTriggerType {
		return req.Body, true, nil
	}
	fields, err := v.fields(ctx, r, node)
	if err != nil {
		return nil, false, err
	}
	secret := fields[v.trigger.Verify.Field]
	if secret == "" {
		return nil, false, fmt.Errorf("the credential of node %s has no %s field", node.ID, v.trigger.Verify.Field)
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
// of a WhatsApp route; every other route declares none and answers [trigger.ErrNoHandshake].
func (v whatsappVerifier) Challenge(ctx context.Context, r trigger.Resolved, req trigger.IngressRequest) (string, error) {
	node, err := v.node(ctx, r)
	if err != nil {
		return "", err
	}
	if node.Type != whatsappTriggerType {
		return "", trigger.ErrNoHandshake
	}
	fields, err := v.fields(ctx, r, node)
	if err != nil {
		return "", err
	}
	return declarative.CheckHandshake(*v.trigger.Handshake, req.Query, fields[v.trigger.Handshake.Field])
}

// fields reads the stored credential the trigger node names, for the project of the route.
func (v whatsappVerifier) fields(ctx context.Context, r trigger.Resolved, node *flow.Node) (map[string]string, error) {
	param, ok := node.Parameters.Get(credentialParameter)
	name, isText := param.Str()
	if !ok || !isText || name == "" {
		return nil, fmt.Errorf("node %s names no %s", node.ID, credentialParameter)
	}
	return v.credentials(ctx, r.Org, r.ProjectID, name)
}

// node reads the trigger node of the route from the active version of its flow.
func (v whatsappVerifier) node(ctx context.Context, r trigger.Resolved) (*flow.Node, error) {
	rec, err := v.flows.GetFlow(ctx, r.Scope, r.FlowID)
	if err != nil {
		return nil, fmt.Errorf("read the flow of the route: %w", err)
	}
	ver, err := v.flows.GetVersion(ctx, r.Scope, rec.ActiveVersionID)
	if err != nil {
		return nil, fmt.Errorf("read the live version of the route: %w", err)
	}
	doc, err := (flowReader{}).Read(ver.Document)
	if err != nil {
		return nil, err
	}
	for i := range doc.Nodes {
		if doc.Nodes[i].ID == r.NodeID {
			return &doc.Nodes[i], nil
		}
	}
	return nil, fmt.Errorf("the live version has no node %s", r.NodeID)
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
