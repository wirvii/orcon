// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
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
