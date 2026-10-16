// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"time"

	"github.com/wirvii/orcon/internal/platform/id"
)

// LastExecution is the most recent execution of a flow, as a list of flows shows it: enough to
// print its status without reading the execution (SPEC-019 R-UX-BE-01). It is read-only.
type LastExecution struct {
	// ID is the identifier of the execution.
	ID id.UUID
	// Status is the execution status, one of the UPPER_SNAKE values of the execution table.
	Status string
	// HasUncertainOperation is true when the execution has an external operation whose outcome
	// is unknown (status UNCERTAIN), so a person must review it (SPEC-019 R-UX-BE-03).
	HasUncertainOperation bool
	CreatedAt             time.Time
	// FinishedAt is the zero time while the execution has not reached a final status.
	FinishedAt time.Time
}

// Activity is a flow with the read-only activity a list shows beside it.
type Activity struct {
	Flow Record
	// LastExecution is nil when the flow never ran.
	LastExecution *LastExecution
	// ExecutionCountLast24h counts the executions accepted in the 24 hours before the read.
	ExecutionCountLast24h int64
}
