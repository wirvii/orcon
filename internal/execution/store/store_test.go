// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"testing"
	"time"

	_ "github.com/wirvii/orcon/internal/execution/executiontest" // defines the flags the criteria commands pass to every package
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

func TestIdentityTypes(t *testing.T) {
	u := id.NewSequence().New()
	text := u.String()
	for name, got := range map[string]string{
		"execution": store.ExecutionID(u).String(), "token": store.TokenID(u).String(), "node execution": store.NodeExecutionID(u).String(),
		"operation": store.ExternalOperationID(u).String(), "event": store.EventID(u).String(), "join instance": store.JoinInstanceID(u).String(),
		"join arrival": store.JoinArrivalID(u).String(), "timer": store.TimerID(u).String(), "wait": store.WaitID(u).String(),
		"outbox": store.OutboxID(u).String(), "attention request": store.AttentionRequestID(u).String(),
	} {
		if got != text {
			t.Errorf("%s identity prints %s, want %s", name, got, text)
		}
	}
	if store.ExecutionID(u).IsZero() || !(store.ExecutionID{}).IsZero() || store.TokenID(u).IsZero() || !(store.TokenID{}).IsZero() ||
		store.NodeExecutionID(u).IsZero() || !(store.NodeExecutionID{}).IsZero() || store.WaitID(u).IsZero() || !(store.WaitID{}).IsZero() {
		t.Error("IsZero")
	}
}

func TestCursor(t *testing.T) {
	if !(store.Cursor{}).IsZero() {
		t.Error("the zero cursor starts the first page")
	}
	if (store.Cursor{CreatedAt: time.Unix(1, 0)}).IsZero() || (store.Cursor{ID: id.NewSequence().New()}).IsZero() {
		t.Error("a cursor with a position is not the first page")
	}
}

// TestExecutionFilterShapes: a filter names a project, or exactly one of flow and status; the
// refinements (flow, status, creation window, order) need the project.
func TestExecutionFilterShapes(t *testing.T) {
	ids := id.NewSequence()
	project := identity.ProjectID(ids.New())
	flowID := flow.FlowID(ids.New())
	at := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		filter  store.ExecutionFilter
		valid   bool
		refined bool
	}{
		{"empty", store.ExecutionFilter{}, false, false},
		{"project", store.ExecutionFilter{ProjectID: project}, true, false},
		{"flow", store.ExecutionFilter{FlowID: flowID}, true, false},
		{"status", store.ExecutionFilter{Status: "FAILED"}, true, false},
		{"flow and status", store.ExecutionFilter{FlowID: flowID, Status: "FAILED"}, false, false},
		{"project and flow", store.ExecutionFilter{ProjectID: project, FlowID: flowID}, true, true},
		{"project and status", store.ExecutionFilter{ProjectID: project, Status: "FAILED"}, true, true},
		{"project and window", store.ExecutionFilter{ProjectID: project, CreatedAfter: at, CreatedBefore: at.Add(time.Hour)}, true, true},
		{"project oldest first", store.ExecutionFilter{ProjectID: project, Oldest: true}, true, true},
		{"status and window", store.ExecutionFilter{Status: "FAILED", CreatedAfter: at}, false, false},
		{"status oldest first", store.ExecutionFilter{Status: "FAILED", Oldest: true}, false, false},
	}
	for _, c := range cases {
		if got := c.filter.Valid(); got != c.valid {
			t.Errorf("%s: Valid = %v, want %v", c.name, got, c.valid)
		}
		if got := c.filter.Refined(); got != c.refined {
			t.Errorf("%s: Refined = %v, want %v", c.name, got, c.refined)
		}
	}
}
