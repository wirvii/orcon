// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// ProjectActivity implements [flow.ActivityStore] (SPEC-019 R-UX-BE-02): the flows of the project, its
// executions since day by hour, status and mode, its uncertain executions since day and its successful
// time-saved executions since week, all read in one transaction.
func (s *Store) ProjectActivity(ctx context.Context, org orgscope.Org, project identity.ProjectID, day, week time.Time) (flow.ProjectActivity, error) {
	var out flow.ProjectActivity
	err := s.read(ctx, "project activity", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		pid := uuidBytes(project)
		if out.Flows, err = q.CountFlows(ctx, scope, pid); err != nil {
			return err
		}
		if out.ActiveFlows, err = q.CountActiveFlows(ctx, scope, pid); err != nil {
			return err
		}
		rows, err := q.ListExecutionHourCounts(ctx, scope, pid, micros(day))
		if err != nil {
			return err
		}
		for i := range rows {
			out.Executions = append(out.Executions, flow.ExecutionCount{
				Hour: instant(rows[i].HourStartedAt), Status: rows[i].Status, Mode: rows[i].Mode, Count: rows[i].ExecutionCount,
			})
		}
		if out.Uncertain, err = q.CountUncertainExecutions(ctx, scope, pid, micros(day)); err != nil {
			return err
		}
		out.TriggerSucceeded, err = q.CountSucceededTriggerExecutions(ctx, scope, pid, micros(week))
		return err
	})
	return out, err
}

// TimeSavedMinutes implements [flow.ActivityStore]: the minutes the organization saves per successful
// automated execution (the column of the organization, 3 by default).
func (s *Store) TimeSavedMinutes(ctx context.Context, org orgscope.Org) (int64, error) {
	var minutes int64
	err := s.read(ctx, "time saved minutes", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		minutes, err = q.GetTimeSavedMinutes(ctx, scope)
		return err
	})
	return minutes, err
}
