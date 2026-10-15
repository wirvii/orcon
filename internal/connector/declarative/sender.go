// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import "context"

// HTTPRequest is one fully rendered request, ready for the network. Header
// keys are canonical names; the sender adds nothing the interpreter did not.
type HTTPRequest struct {
	Method string
	// URL is absolute and already carries the query string.
	URL    string
	Header map[string][]string
	Body   []byte
}

// HTTPResponse is the answer of the provider. Header keys are lower case.
type HTTPResponse struct {
	Status int
	Header map[string][]string
	Body   []byte
}

// Sender is the network port of the interpreter. Its implementation applies
// the network policy of the connector (ADR-0029 §8) and returns an error that
// wraps [ErrEgressDenied] or [ErrNotSent] when the request never left the host.
// Any other error means the provider may have received the request.
type Sender interface {
	Send(ctx context.Context, req HTTPRequest) (HTTPResponse, error)
}
