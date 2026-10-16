// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"strconv"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// acceptedStatus is the hourly counter row of the executions accepted in an hour (see migration 0025).
const acceptedStatus = "ACCEPTED"

// isTerminalStatus reports whether an execution status is final.
func isTerminalStatus(status string) bool {
	switch status {
	case "SUCCEEDED", "FAILED", "CANCELED":
		return true
	}
	return false
}

// hourOf is the start of the hour, in microseconds, that an execution accepted at acceptedAt counts in.
func hourOf(acceptedAt time.Time) int64 { return micros(acceptedAt.UTC().Truncate(time.Hour)) }

// countHour adds one execution to the hourly counter of its project under the status given: ACCEPTED
// for an acceptance, or the terminal status it reached. The hour is the one the execution was accepted
// in, so the counters agree with a recount of the execution table. It runs in the caller's transaction.
func countHour(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, project identity.ProjectID, acceptedAt time.Time, mode, status string, now time.Time) error {
	hour := hourOf(acceptedAt)
	return q.UpsertExecutionHourCount(ctx, sqlcgen.UpsertExecutionHourCountParams{
		ID:    uuidBytes(id.Derive("execution_hour_count", org.String(), project.String(), strconv.FormatInt(hour, 10), mode, status)),
		OrgID: uuidBytes(org), ProjectID: uuidBytes(project),
		HourStartedAt: hour, Mode: mode, Status: status, Now: micros(now),
	})
}

// uncountHour removes one execution from the hourly counter it left; the counter of an execution that
// reopened after a final status (an in-place retry) must match a recount of the execution table.
func uncountHour(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, project identity.ProjectID, acceptedAt time.Time, mode, status string, now time.Time) error {
	_, err := q.UpdateExecutionHourCountDown(ctx, sqlcgen.UpdateExecutionHourCountDownParams{
		Now: micros(now), OrgID: uuidBytes(org), ProjectID: uuidBytes(project),
		HourStartedAt: hourOf(acceptedAt), Mode: mode, Status: status,
	})
	return err
}

// countAcceptance counts an execution accepted by InsertExecution: under ACCEPTED, and also under its
// status when the row is inserted already final.
func countAcceptance(ctx context.Context, q *sqlcgen.Queries, e *store.Execution) error {
	if err := countHour(ctx, q, e.OrgID, e.ProjectID, e.CreatedAt, e.Mode, acceptedStatus, e.CreatedAt); err != nil {
		return err
	}
	if isTerminalStatus(e.Status) {
		return countHour(ctx, q, e.OrgID, e.ProjectID, e.CreatedAt, e.Mode, e.Status, e.CreatedAt)
	}
	return nil
}

// countKey is what the counters need of an execution before its status changes: the project, the mode,
// the status it has and the instant that decides its hour. It is read with a narrow query, not the row.
type countKey struct {
	project identity.ProjectID
	status  string
	mode    string
	created time.Time
}

// readCountKey reads the countKey of an execution; found is false when the execution does not exist.
func readCountKey(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, exec store.ExecutionID) (key countKey, found bool, err error) {
	row, err := q.GetExecutionCountKey(ctx, uuidBytes(org), uuidBytes(exec))
	if isNoRows(err) {
		return countKey{}, false, nil
	}
	if err != nil {
		return countKey{}, false, err
	}
	project, err := uuidOf[identity.ProjectID](row.ProjectID)
	if err != nil {
		return countKey{}, false, err
	}
	return countKey{project: project, status: row.Status, mode: row.Mode, created: instant(row.CreatedAt)}, true, nil
}

// countTransition keeps the counters equal to a recount after an execution changed status: the counter
// of a final status it leaves goes down, the counter of a final status it reaches goes up. A change that
// neither leaves nor reaches a final status touches nothing.
func countTransition(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, prior countKey, status string, now time.Time) error {
	if prior.status == status {
		return nil
	}
	if isTerminalStatus(prior.status) {
		if err := uncountHour(ctx, q, org, prior.project, prior.created, prior.mode, prior.status, now); err != nil {
			return err
		}
	}
	if isTerminalStatus(status) {
		return countHour(ctx, q, org, prior.project, prior.created, prior.mode, status, now)
	}
	return nil
}
