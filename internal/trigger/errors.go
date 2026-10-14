// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"errors"
	"fmt"
)

// Errors of the area. Storage errors keep the identity they have in the flow
// area, such as flow.ErrNotFound and flow.ErrConflict, and the service wraps them.
var (
	// ErrInactive means the flow or the trigger does not accept events: the
	// flow is inactive or deleted, or the node left the live version. It is the
	// reason `INACTIVE` of ingress.rejected (R-OBS-01).
	ErrInactive = errors.New("trigger is inactive")
	// ErrNotPublished means the flow has no published version, so there is
	// nothing to activate or to run.
	ErrNotPublished = errors.New("flow has no published version")
	// ErrFlowDeleted means the operation needs a flow that is not deleted.
	ErrFlowDeleted = errors.New("flow is deleted")
	// ErrNotDeleted means the operation needs a deleted flow.
	ErrNotDeleted = errors.New("flow is not deleted")
	// ErrRecoveryWindow means a deleted flow is still inside its 30-day
	// recovery window and cannot be purged yet.
	ErrRecoveryWindow = errors.New("flow is inside its recovery window")
	// ErrRecoveryExpired means a deleted flow is past its 30-day recovery
	// window and can no longer be restored.
	ErrRecoveryExpired = errors.New("flow is past its recovery window")
	// ErrNoStartNode means a manual run found no start event, or several and
	// no choice.
	ErrNoStartNode = errors.New("flow has no single start event to run")
	// ErrRouteUnavailable is the generic problem for a path that belongs to
	// another organization. It carries no detail about that organization
	// (ADR-0022 section 3).
	ErrRouteUnavailable = errors.New("path unavailable")
)

// ConfigError reports a service that is missing a dependency.
type ConfigError struct{ Field string }

// Error names the missing field.
func (e *ConfigError) Error() string {
	return fmt.Sprintf("trigger service: %s is required", e.Field)
}
