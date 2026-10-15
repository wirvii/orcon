// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/wirvii/orcon/internal/connector/declarative"
)

// Sender is the network port of the declarative interpreter bound to one
// policy. It sends every request through [Client.Do], so the network rules of
// this package apply to each hop.
type Sender struct {
	client *Client
	policy Policy
}

// Sender returns a [declarative.Sender] that enforces policy.
func (c *Client) Sender(policy Policy) *Sender {
	return &Sender{client: c, policy: policy}
}

var _ declarative.Sender = (*Sender)(nil)

// Send performs req and reads the whole response body. A refused destination
// is reported as [declarative.ErrEgressDenied]. A request that cannot be built
// is reported as [declarative.ErrNotSent]: nothing left the host. Every other
// error may follow a transmission, and the interpreter treats it as ambiguous.
func (s *Sender) Send(ctx context.Context, in declarative.HTTPRequest) (declarative.HTTPResponse, error) {
	req, err := http.NewRequestWithContext(ctx, in.Method, in.URL, bytes.NewReader(in.Body))
	if err != nil {
		return declarative.HTTPResponse{}, fmt.Errorf("%w: the request could not be built", declarative.ErrNotSent)
	}
	for k, vs := range in.Header {
		req.Header[k] = append([]string(nil), vs...)
	}
	resp, err := s.client.Do(ctx, s.policy, req)
	if err != nil {
		var denied *DeniedError
		if errors.As(err, &denied) {
			return declarative.HTTPResponse{}, fmt.Errorf("%w: %s", declarative.ErrEgressDenied, denied.Reason)
		}
		return declarative.HTTPResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return declarative.HTTPResponse{}, fmt.Errorf("reading the response body: %w", err)
	}
	header := make(map[string][]string, len(resp.Header))
	for k, vs := range resp.Header {
		lk := strings.ToLower(k)
		header[lk] = append(header[lk], vs...)
	}
	return declarative.HTTPResponse{Status: resp.StatusCode, Header: header, Body: body}, nil
}
