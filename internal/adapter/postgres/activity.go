// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// ProjectActivity implements [flow.ActivityStore] (SPEC-019 R-UX-BE-02): the flows of the project, its
// executions since day by hour, status and mode, its uncertain executions since day and its successful
// time-saved executions since week, all read in one transaction.
func (s *Store) ProjectActivity(ctx context.Context, org orgscope.Org, project identity.ProjectID, day, week time.Time) (flow.ProjectActivity, error) {
	var out flow.ProjectActivity
	err := s.inScope(ctx, "project activity", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		orgID, pid := pgUUID(org.UUID()), pgUUID(project)
		var err error
		if out.Flows, err = q.CountFlows(ctx, orgID, pid); err != nil {
			return err
		}
		if out.ActiveFlows, err = q.CountActiveFlows(ctx, orgID, pid); err != nil {
			return err
		}
		rows, err := q.ListExecutionHourCounts(ctx, orgID, pid, timestamp(day))
		if err != nil {
			return err
		}
		for i := range rows {
			out.Executions = append(out.Executions, flow.ExecutionCount{
				Hour: instant(rows[i].HourStartedAt), Status: rows[i].Status, Mode: rows[i].Mode, Count: rows[i].ExecutionCount,
			})
		}
		if out.Uncertain, err = q.CountUncertainExecutions(ctx, orgID, pid, timestamp(day)); err != nil {
			return err
		}
		out.TriggerSucceeded, err = q.CountSucceededTriggerExecutions(ctx, orgID, pid, timestamp(week))
		return err
	})
	return out, err
}

// TimeSavedMinutes implements [flow.ActivityStore]: the minutes the organization saves per successful
// automated execution (the column of the organization, 3 by default).
func (s *Store) TimeSavedMinutes(ctx context.Context, org orgscope.Org) (int64, error) {
	var minutes int64
	err := s.inScope(ctx, "time saved minutes", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.GetTimeSavedMinutes(ctx, pgUUID(org.UUID()))
		minutes = int64(n)
		return err
	})
	return minutes, err
}
