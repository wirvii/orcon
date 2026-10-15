// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"context"
	"errors"
	"fmt"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// ErrBadBody means the body of a public request is not JSON, so it cannot
// become the items of an execution.
var ErrBadBody = errors.New("the body is not JSON")

// ErrUnauthorized means a public request did not prove that its sender knows
// the trigger's secret: the signature is missing or wrong, or the handshake
// token is wrong. Nothing is started and the caller learns nothing more.
var ErrUnauthorized = errors.New("the request is not authenticated")

// IngressRequest is a public request addressed to a trigger: the method and
// the path it came to, its headers and query, and its raw body. Headers are
// read by the verifier only; the body is the event when it is accepted.
type IngressRequest struct {
	Method  string
	Path    string
	Headers map[string][]string
	Query   map[string]string
	Body    []byte
}

// Verifier authenticates public requests against the verification a trigger
// declares. The request is checked before the acceptance is read, so a refused
// request never starts an execution.
type Verifier interface {
	// Verify returns an error wrapping [ErrUnauthorized] unless the request
	// carries the signature the route's trigger declares.
	Verify(ctx context.Context, r Resolved, req IngressRequest) error
	// Challenge answers a subscription handshake: it returns the text to echo,
	// or an error wrapping [ErrUnauthorized] when the token is wrong or the
	// trigger declares no handshake.
	Challenge(ctx context.Context, r Resolved, req IngressRequest) (string, error)
}

// Accepted is a request the ingress accepted: the execution it started. The
// execution runs on the version the registry named when the request arrived.
type Accepted struct {
	ExecutionID string
}

// Ingest accepts one public request for a webhook trigger (SPEC-002 T2, first
// cut). It resolves the address with [Store.ResolveRoute] without knowing the
// organization, reads the acceptance in that organization (rules of
// [Service.Accept]: the flow must be active and not deleted, and the node must
// still be live), and starts one execution pinned to the accepted version.
//
// Deduplication, the flow ingress cap, response modes and the ingress audit
// events are not part of this cut; each arrives with its own cut of SPEC-002.
// The acceptance read and the execution start run in two transactions, so a
// request that is accepted and then deactivated can still start one execution.
func (s *Service) Ingest(ctx context.Context, req IngressRequest) (Accepted, error) {
	if s.starter == nil {
		return Accepted{}, ErrNoStarter
	}
	if _, err := jsonvalue.Parse(req.Body); err != nil {
		return Accepted{}, fmt.Errorf("%w: %w", ErrBadBody, err)
	}
	r, err := s.store.ResolveRoute(ctx, req.Method, req.Path)
	if err != nil {
		return Accepted{}, fmt.Errorf("resolve %s %s: %w", req.Method, req.Path, err)
	}
	if s.verifier != nil {
		if err := s.verifier.Verify(ctx, r, req); err != nil {
			return Accepted{}, fmt.Errorf("%w: %w", ErrUnauthorized, err)
		}
	}
	acc, err := s.Accept(ctx, r.Org, r.FlowID, r.NodeID)
	if err != nil {
		return Accepted{}, err
	}
	id, err := s.starter.StartTriggered(ctx, TriggeredRun{
		Org: r.Org, ProjectID: r.ProjectID, FlowID: r.FlowID, Node: r.NodeID,
		Version: acc.FlowVersionID, Trigger: acc.TriggerID,
	}, req.Body)
	if err != nil {
		return Accepted{}, fmt.Errorf("start the execution of node %q: %w", r.NodeID, err)
	}
	return Accepted{ExecutionID: id}, nil
}

// Handshake answers the subscription request of a provider that verifies a
// webhook address with a GET (SPEC-002 T2; ADR-0030 §8). It resolves the GET
// address, asks the verifier for the challenge and returns it. It starts no
// execution and reads no acceptance. Without a verifier nothing can be
// confirmed, so the request is refused.
func (s *Service) Handshake(ctx context.Context, req IngressRequest) (string, error) {
	if s.verifier == nil {
		return "", fmt.Errorf("%w: no verifier is wired", ErrUnauthorized)
	}
	r, err := s.store.ResolveRoute(ctx, req.Method, req.Path)
	if err != nil {
		return "", fmt.Errorf("resolve %s %s: %w", req.Method, req.Path, err)
	}
	challenge, err := s.verifier.Challenge(ctx, r, req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnauthorized, err)
	}
	return challenge, nil
}
