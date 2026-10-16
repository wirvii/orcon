// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
)

// Activity serves getOrgActivitySummary (SPEC-019 R-UX-BE-02): the figures of the home screen of an
// organization, counted over the projects the caller reads.
type Activity struct {
	access   ProjectAccessResolver
	projects identity.ProjectStore
	store    flow.ActivityStore
	clock    clock.Clock
}

// NewActivity returns the handler over the project access resolver, the project listing, the activity
// read side and the clock of the instant the window ends at.
func NewActivity(access ProjectAccessResolver, projects identity.ProjectStore, store flow.ActivityStore, clk clock.Clock) *Activity {
	return &Activity{access: access, projects: projects, store: store, clock: clk}
}

// GetOrgActivitySummary answers the activity summary of the organization the request was authorized
// in. The projects it counts are the ones the caller reads (authz.ReadableProjects).
func (a *Activity) GetOrgActivitySummary(ctx context.Context, _ apigen.GetOrgActivitySummaryRequestObject) (apigen.GetOrgActivitySummaryResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok || principal.UserID.IsZero() {
		return nil, identity.ErrUnauthenticated
	}
	access, err := a.access.ProjectAccess(ctx, principal.UserID, org)
	if err != nil {
		return nil, err
	}
	readable, err := authz.ReadableProjects(ctx, a.projects, org, access, page.MaxLimit, maxListRows)
	if err != nil {
		return nil, err
	}
	projects := make([]identity.ProjectID, 0, len(readable))
	for i := range readable {
		projects = append(projects, readable[i].ID)
	}
	sum, err := flow.ReadActivitySummary(ctx, a.store, org, projects, a.clock.Now())
	if err != nil {
		return nil, err
	}
	return apigen.GetOrgActivitySummary200JSONResponse{Body: activitySummaryBody(&sum)}, nil
}

// activitySummaryBody is the summary as the contract states it.
func activitySummaryBody(s *flow.ActivitySummary) apigen.ActivitySummary {
	byHour := make([]int, flow.SummaryHours)
	for i, n := range s.Executions.ByHour {
		byHour[i] = int(n)
	}
	return apigen.ActivitySummary{
		ActiveFlowCount: countOf(s.ActiveFlowCount),
		FlowCount:       countOf(s.FlowCount),
		ProjectCount:    countOf(s.ProjectCount),
		ExecutionsLast24h: apigen.ActivityExecutionCounts{
			Total: countOf(s.Executions.Total), Failed: countOf(s.Executions.Failed),
			Succeeded: countOf(s.Executions.Succeeded), Uncertain: countOf(s.Executions.Uncertain),
			ByHour: &byHour,
		},
		TimeSaved: apigen.ActivityTimeSaved{
			WindowDays: countOf(s.TimeSaved.WindowDays), SucceededExecutions: countOf(s.TimeSaved.SucceededExecutions),
			MinutesPerExecution: countOf(s.TimeSaved.MinutesPerExecution), Minutes: countOf(s.TimeSaved.Minutes),
		},
	}
}

// countOf is a count as the contract's optional integer field.
func countOf(n int64) *int {
	v := int(n)
	return &v
}
