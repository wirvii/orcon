// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package acmequeue is the official native connector for Acme Queue, the
// example message queue of the connector templates. It is native because
// Acme Queue speaks its own TCP protocol (gap DG-1 in
// docs/reference/declarative-gaps.md; see authoring.json). Copy this folder
// to internal/adapter/connectors/<name>/ and rename the service.
//
// The code follows the orcon-go skill and the SDK contract of ADR-0031:
// reads implement sdk.Reader, writes implement sdk.Effector, and only
// Dispatch may return an uncertain outcome.
package acmequeue

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/wirvii/orcon/packages/go/connector/sdk"
)

// Connector serves every operation of Acme Queue.
type Connector struct{}

var (
	_ sdk.Connector = Connector{}
	_ sdk.Reader    = Connector{}
	_ sdk.Effector  = Connector{}
)

// Manifest returns the connector manifest (manifest.go).
func (Connector) Manifest() sdk.Manifest { return manifest }

// Read serves the READ operations.
func (c Connector) Read(ctx context.Context, call sdk.Call) (sdk.Output, error) {
	switch call.Operation() {
	case "message.getMany":
		return c.getMany(ctx, call)
	default:
		return sdk.Output{}, sdk.Permanent(sdk.CodeUnsupported, fmt.Errorf("operation %q is not a read", call.Operation()))
	}
}

// Prepare turns each input item of a write into one intent, before anything
// is sent. It never talks to the service.
func (Connector) Prepare(_ context.Context, call sdk.Call) ([]sdk.Intent, error) {
	if call.Operation() != "message.send" {
		return nil, sdk.Permanent(sdk.CodeUnsupported, fmt.Errorf("operation %q is not a write", call.Operation()))
	}
	return sdk.IntentPerItem(call)
}

// Dispatch sends one intent. message.send is UNSAFE (no idempotency key):
// once the request may have left, any failure is uncertain and the engine
// asks a person instead of resending (ER-4, TE-6).
func (c Connector) Dispatch(ctx context.Context, call sdk.Call, in sdk.Intent) (sdk.Outcome, error) {
	conn, err := call.Egress().Dial(ctx, "tcp", "queue.acme.example:5671")
	if err != nil {
		// Nothing was sent: the failure is safe to retry.
		return sdk.Outcome{}, sdk.Transient(sdk.CodeNetworkError, fmt.Errorf("connect to Acme Queue: %w", err))
	}
	defer conn.Close()

	if err := writeFrame(ctx, conn, in); err != nil {
		if errors.Is(err, errNotSent) {
			return sdk.Outcome{}, sdk.Transient(sdk.CodeNetworkError, err)
		}
		return sdk.Outcome{}, sdk.Uncertain(sdk.CodeNetworkError, fmt.Errorf("send message: %w", err))
	}
	id, err := readAck(ctx, conn)
	if err != nil {
		return sdk.Outcome{}, sdk.Uncertain(sdk.CodeTimeout, fmt.Errorf("wait for acknowledgement: %w", err))
	}
	return sdk.Succeeded(map[string]any{"id": id}), nil
}

// Reconcile is not used: message.send is UNSAFE and has no verification
// operation. The SDK requires the method on every Effector.
func (Connector) Reconcile(context.Context, sdk.Call, sdk.Intent) (sdk.Outcome, error) {
	return sdk.Outcome{}, sdk.Permanent(sdk.CodeUnsupported, errors.New("message.send cannot be reconciled"))
}

func (Connector) getMany(ctx context.Context, call sdk.Call) (sdk.Output, error) {
	// Read pages until returnAll is satisfied or limit is reached, streaming
	// each page to the output (PG-3); honour ctx cancellation between pages.
	_ = ctx
	_ = call
	return sdk.Output{}, nil
}

var errNotSent = errors.New("request not sent")

func writeFrame(ctx context.Context, conn net.Conn, in sdk.Intent) error {
	// Encode the intent as an Acme Queue frame and write it. Return
	// errNotSent only when no byte reached the connection.
	_, _, _ = ctx, conn, in
	return nil
}

func readAck(ctx context.Context, conn net.Conn) (string, error) {
	_, _ = ctx, conn
	return "", nil
}
