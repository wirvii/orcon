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

// IngressRequest is a public request addressed to a trigger: the method and
// the path it came to, and its raw body.
type IngressRequest struct {
	Method string
	Path   string
	Body   []byte
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
