// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/wirvii/orcon/internal/flow"
)

// routePageSize is the page the collision check reads the routes of the
// organization with.
const routePageSize = 500

// claim is one (path, method) pair a document asks for.
type claim struct {
	node    flow.NodeID
	path    string
	pattern string
	method  string
}

// routeCheck is the input of the T-V1 check.
type routeCheck struct {
	flowID flow.FlowID
	doc    *flow.Document
	specs  []Spec
	now    time.Time
	// lock takes the cross-organization lock of every claimed path; publication
	// does, a dry-run validation does not.
	lock bool
}

// claims lists the production addresses the document asks for, with the path
// of each in the document.
func (c *routeCheck) claims() []claim {
	byNode := SpecsByNode(c.specs)
	var out []claim
	for i := range c.doc.Nodes {
		n := &c.doc.Nodes[i]
		spec, ok := byNode[n.ID]
		if n.Trigger == nil || !ok {
			continue
		}
		for j, a := range spec.Addresses {
			for _, m := range a.Methods {
				out = append(out, claim{
					node: n.ID, path: fmt.Sprintf("/nodes/%d/trigger/addresses/%d", i, j),
					pattern: a.Pattern, method: m,
				})
			}
		}
	}
	return out
}

// routeIssues is rule T-V1 (ADR-0022 section 3, ADR-0024 section 2): the paths
// a document asks for must be free. A path held by another trigger of the same
// organization is an error that names the owning flow. A path held by another
// organization is the generic error "path unavailable" with no detail, found
// by a question to the store that answers only yes or no. Patterns that can
// match the same request as a path of the same organization are a warning.
func (s *Service) routeIssues(ctx context.Context, tx Tx, c *routeCheck) ([]flow.Issue, error) {
	claims := c.claims()
	if len(claims) == 0 {
		return nil, nil
	}
	issues := duplicateClaimIssues(claims)
	if c.lock {
		if err := lockClaims(ctx, tx, claims); err != nil {
			return nil, err
		}
	}
	routes, err := s.organizationRoutes(ctx, tx)
	if err != nil {
		return nil, err
	}
	graceStart := c.now.Add(-RetiredGrace)
	for i := range claims {
		cl := &claims[i]
		issues = append(issues, sameOrganizationIssues(cl, routes, c.flowID, graceStart)...)
		taken, err := tx.RouteTakenByOtherOrganization(ctx, ScopeProduction, cl.pattern, cl.method, graceStart)
		if err != nil {
			return nil, err
		}
		if taken {
			issues = append(issues, routeIssue(flow.SeverityError, cl.path, "%s %s: %s", cl.method, cl.pattern, ErrRouteUnavailable))
		}
	}
	return dedupeIssues(issues), nil
}

func routeIssue(sev flow.Severity, path, format string, args ...any) flow.Issue {
	return flow.Issue{Severity: sev, Code: CodeTV1, Path: path, Message: fmt.Sprintf(format, args...)}
}

// duplicateClaimIssues reports two nodes of one document asking for the same
// path and method.
func duplicateClaimIssues(claims []claim) []flow.Issue {
	type key struct{ pattern, method string }
	first := map[key]flow.NodeID{}
	var issues []flow.Issue
	for i := range claims {
		cl := &claims[i]
		k := key{cl.pattern, cl.method}
		if other, dup := first[k]; dup && other != cl.node {
			issues = append(issues, routeIssue(flow.SeverityError, cl.path, "%s %s is asked for by nodes %q and %q of this flow", cl.method, cl.pattern, other, cl.node))
			continue
		}
		first[k] = cl.node
	}
	return issues
}

// lockClaims takes the cross-organization lock of every claimed path, in one
// order for every publication so that two of them cannot wait for each other.
func lockClaims(ctx context.Context, tx Tx, claims []claim) error {
	patterns := make([]string, 0, len(claims))
	for i := range claims {
		if !slices.Contains(patterns, claims[i].pattern) {
			patterns = append(patterns, claims[i].pattern)
		}
	}
	slices.Sort(patterns)
	for _, p := range patterns {
		if err := tx.LockRoute(ctx, ScopeProduction, p); err != nil {
			return err
		}
	}
	return nil
}

// sameOrganizationIssues compares a claim with the routes of the organization:
// the same path and method held by another trigger is an error that names its
// flow, and a pattern that can match the same requests is a warning.
func sameOrganizationIssues(cl *claim, routes []Route, own flow.FlowID, graceStart time.Time) []flow.Issue {
	var issues []flow.Issue
	for i := range routes {
		r := &routes[i]
		if (r.FlowID == own && r.NodeID == cl.node) || r.Method != cl.method {
			continue // the trigger's own address, or another method
		}
		if r.Status == AddressRetired && r.RetiredAt.Before(graceStart) {
			continue
		}
		switch {
		case r.Pattern == cl.pattern:
			issues = append(issues, routeIssue(flow.SeverityError, cl.path, "%s %s is already used by flow %q (node %q)", cl.method, cl.pattern, r.FlowName, r.NodeID))
		case Overlaps(r.Pattern, cl.pattern):
			issues = append(issues, routeIssue(flow.SeverityWarning, cl.path, "%s %s can match the same requests as %s of flow %q; the pattern with more static segments wins", cl.method, cl.pattern, r.Pattern, r.FlowName))
		}
	}
	return issues
}

// organizationRoutes reads every (address, method) pair of the organization in
// the production scope, page by page.
func (s *Service) organizationRoutes(ctx context.Context, tx Tx) ([]Route, error) {
	var out []Route
	var after MethodID
	for {
		page, err := tx.ListRoutes(ctx, ScopeProduction, after, routePageSize)
		if err != nil {
			return nil, err
		}
		for i := range page {
			out = append(out, page[i].Route)
			after = page[i].MethodID
		}
		if len(page) < routePageSize {
			return out, nil
		}
	}
}

// dedupeIssues drops repeated findings, which the per-method claims produce
// for a path with several methods.
func dedupeIssues(in []flow.Issue) []flow.Issue {
	seen := map[flow.Issue]bool{}
	out := in[:0]
	for _, i := range in {
		if !seen[i] {
			seen[i] = true
			out = append(out, i)
		}
	}
	return out
}
