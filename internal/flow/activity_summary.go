// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The window of the activity summary (SPEC-019 R-UX-BE-02): the executions of the last 24 hours, counted
// by hour from the start of the first hour, and the successful automated executions of the last week.
const (
	SummaryHours      = 24
	SummaryWindowDays = 7
)

// AcceptedStatus is the status of the counter row that counts the executions accepted in an hour; the
// other statuses count the executions accepted in an hour that reached a final status.
const AcceptedStatus = "ACCEPTED"

// ExecutionCount counts the executions of one project that were accepted in one hour, under a status
// (AcceptedStatus or a final status) and a mode.
type ExecutionCount struct {
	// Hour is the start of the hour the executions were accepted in, in UTC.
	Hour   time.Time
	Status string
	Mode   string
	Count  int64
}

// ProjectActivity is the raw activity of one project that the summary folds together.
type ProjectActivity struct {
	// Flows counts the flows that are not deleted; ActiveFlows counts those that are active.
	Flows       int64
	ActiveFlows int64
	// Executions are the executions accepted since the start of the day, by hour, status and mode.
	Executions []ExecutionCount
	// Uncertain counts the executions accepted since the start of the day that have an external operation
	// whose outcome is unknown (the executions that need a review).
	Uncertain int64
	// TriggerSucceeded counts the executions of the time-saved mode that succeeded since the start of the week.
	TriggerSucceeded int64
}

// ActivityStore is the read side of the organization activity summary (SPEC-019 R-UX-BE-02). Each read
// is scoped to the organization and happens in one transaction of the adapter.
type ActivityStore interface {
	// ProjectActivity reads the activity of one project: its flows, the executions accepted at or after
	// day, and the successful executions of the time-saved mode accepted at or after week.
	ProjectActivity(ctx context.Context, org orgscope.Org, project identity.ProjectID, day, week time.Time) (ProjectActivity, error)
	// TimeSavedMinutes returns the minutes the organization saves per successful automated execution.
	TimeSavedMinutes(ctx context.Context, org orgscope.Org) (int64, error)
}

// ActivityExecutions are the executions of the summary window.
type ActivityExecutions struct {
	Total     int64
	Failed    int64
	Succeeded int64
	// Uncertain counts the executions that have an operation whose outcome is unknown.
	Uncertain int64
	// ByHour counts the executions per hour of the window, the oldest hour first; the last item is the current hour.
	ByHour [SummaryHours]int64
}

// ActivityTimeSaved is the time the organization saves by its automated executions in the last week.
type ActivityTimeSaved struct {
	WindowDays          int64
	SucceededExecutions int64
	MinutesPerExecution int64
	Minutes             int64
}

// ActivitySummary is what the home screen of an organization shows. It counts only the projects the
// caller reads (the caller decides which projects it passes to [ReadActivitySummary]).
type ActivitySummary struct {
	ActiveFlowCount int64
	FlowCount       int64
	ProjectCount    int64
	Executions      ActivityExecutions
	TimeSaved       ActivityTimeSaved
}

// SummaryDay returns the start of the summary window that ends at now: the start of the hour that
// is 23 hours before the current hour, in UTC.
func SummaryDay(now time.Time) time.Time {
	return now.UTC().Truncate(time.Hour).Add(-(SummaryHours - 1) * time.Hour)
}

// SummaryWeek returns the start of the time-saved window that ends at now: the first hour of the seven
// days, counting the current hour as the last one.
func SummaryWeek(now time.Time) time.Time {
	return now.UTC().Truncate(time.Hour).Add(-(SummaryWindowDays*24 - 1) * time.Hour)
}

// Summarize folds the activity of the projects into the summary. The day argument is the start of the
// window (see [SummaryDay]). The minutesPerExecution argument is the organization's time-saved setting.
func Summarize(day time.Time, minutesPerExecution int64, projects []ProjectActivity) ActivitySummary {
	out := ActivitySummary{
		ProjectCount: int64(len(projects)),
		TimeSaved:    ActivityTimeSaved{WindowDays: SummaryWindowDays, MinutesPerExecution: minutesPerExecution},
	}
	for i := range projects {
		p := &projects[i]
		out.FlowCount += p.Flows
		out.ActiveFlowCount += p.ActiveFlows
		out.Executions.Uncertain += p.Uncertain
		out.TimeSaved.SucceededExecutions += p.TriggerSucceeded
		for _, c := range p.Executions {
			switch c.Status {
			case AcceptedStatus:
				out.Executions.Total += c.Count
				if offset := int(c.Hour.Sub(day) / time.Hour); offset >= 0 && offset < SummaryHours {
					out.Executions.ByHour[offset] += c.Count
				}
			case "FAILED":
				out.Executions.Failed += c.Count
			case "SUCCEEDED":
				out.Executions.Succeeded += c.Count
			}
		}
	}
	out.TimeSaved.Minutes = out.TimeSaved.SucceededExecutions * minutesPerExecution
	return out
}

// ReadActivitySummary reads the summary of an organization over the projects given (the projects the
// caller reads), at the instant now. It makes one [ActivityStore.ProjectActivity] call per project and
// one [ActivityStore.TimeSavedMinutes] call; no call depends on the number of executions.
func ReadActivitySummary(ctx context.Context, store ActivityStore, org orgscope.Org, projects []identity.ProjectID, now time.Time) (ActivitySummary, error) {
	day, week := SummaryDay(now), SummaryWeek(now)
	minutes, err := store.TimeSavedMinutes(ctx, org)
	if err != nil {
		return ActivitySummary{}, err
	}
	activity := make([]ProjectActivity, 0, len(projects))
	for _, project := range projects {
		a, err := store.ProjectActivity(ctx, org, project, day, week)
		if err != nil {
			return ActivitySummary{}, err
		}
		activity = append(activity, a)
	}
	return Summarize(day, minutes, activity), nil
}
