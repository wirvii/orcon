// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"errors"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
)

// errUnavailable is what an operation without a handler returns. The error
// handler of the router turns it into 503 service-unavailable (ERR-6).
var errUnavailable = errors.New("the operation has no handler yet")

// Unavailable implements every operation of the contract by refusing it with
// 503 service-unavailable. It is the stand-in for operations whose handlers
// are not written yet (SPEC-005 Amendment 6): each pull request that
// implements operations removes them from this file, and the file disappears
// when the last one has a handler. No operation may still answer from here in
// the first public release.
type Unavailable struct{}

var _ apigen.StrictServerInterface = Unavailable{}

// CreateSession refuses POST /users/me/sessions.
func (Unavailable) CreateSession(context.Context, apigen.CreateSessionRequestObject) (apigen.CreateSessionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// RevokeSession refuses POST /users/me/sessions/{sessionId}/revoke.
func (Unavailable) RevokeSession(context.Context, apigen.RevokeSessionRequestObject) (apigen.RevokeSessionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// GetCurrentUser refuses GET /users/me.
func (Unavailable) GetCurrentUser(context.Context, apigen.GetCurrentUserRequestObject) (apigen.GetCurrentUserResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ListOrgs refuses GET /orgs.
func (Unavailable) ListOrgs(context.Context, apigen.ListOrgsRequestObject) (apigen.ListOrgsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ListOrgFlows refuses GET /orgs/{orgId}/flows.
func (Unavailable) ListOrgFlows(context.Context, apigen.ListOrgFlowsRequestObject) (apigen.ListOrgFlowsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// GetExecution refuses GET /executions/{executionId}.
func (Unavailable) GetExecution(context.Context, apigen.GetExecutionRequestObject) (apigen.GetExecutionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// CancelExecution refuses POST /executions/{executionId}/cancel.
func (Unavailable) CancelExecution(context.Context, apigen.CancelExecutionRequestObject) (apigen.CancelExecutionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ListExecutionData refuses GET /executions/{executionId}/data.
func (Unavailable) ListExecutionData(context.Context, apigen.ListExecutionDataRequestObject) (apigen.ListExecutionDataResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ListExecutionEvents refuses GET /executions/{executionId}/events.
func (Unavailable) ListExecutionEvents(context.Context, apigen.ListExecutionEventsRequestObject) (apigen.ListExecutionEventsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ListExecutionNodes refuses GET /executions/{executionId}/nodes.
func (Unavailable) ListExecutionNodes(context.Context, apigen.ListExecutionNodesRequestObject) (apigen.ListExecutionNodesResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// RetryExecution refuses POST /executions/{executionId}/retry.
func (Unavailable) RetryExecution(context.Context, apigen.RetryExecutionRequestObject) (apigen.RetryExecutionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ResolveExternalOperation refuses POST /external-operations/{externalOperationId}/resolve.
func (Unavailable) ResolveExternalOperation(context.Context, apigen.ResolveExternalOperationRequestObject) (apigen.ResolveExternalOperationResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// DeleteFlow refuses DELETE /flows/{flowId}.
func (Unavailable) DeleteFlow(context.Context, apigen.DeleteFlowRequestObject) (apigen.DeleteFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// GetFlow refuses GET /flows/{flowId}.
func (Unavailable) GetFlow(context.Context, apigen.GetFlowRequestObject) (apigen.GetFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// UpdateFlow refuses PATCH /flows/{flowId}.
func (Unavailable) UpdateFlow(context.Context, apigen.UpdateFlowRequestObject) (apigen.UpdateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ActivateFlow refuses POST /flows/{flowId}/activate.
func (Unavailable) ActivateFlow(context.Context, apigen.ActivateFlowRequestObject) (apigen.ActivateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// DeactivateFlow refuses POST /flows/{flowId}/deactivate.
func (Unavailable) DeactivateFlow(context.Context, apigen.DeactivateFlowRequestObject) (apigen.DeactivateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// GetDraft refuses GET /flows/{flowId}/draft.
func (Unavailable) GetDraft(context.Context, apigen.GetDraftRequestObject) (apigen.GetDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// UpdateDraft refuses PATCH /flows/{flowId}/draft.
func (Unavailable) UpdateDraft(context.Context, apigen.UpdateDraftRequestObject) (apigen.UpdateDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ReplaceDraft refuses PUT /flows/{flowId}/draft.
func (Unavailable) ReplaceDraft(context.Context, apigen.ReplaceDraftRequestObject) (apigen.ReplaceDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ValidateDraft refuses POST /flows/{flowId}/draft/validate.
func (Unavailable) ValidateDraft(context.Context, apigen.ValidateDraftRequestObject) (apigen.ValidateDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// CreateExecution refuses POST /flows/{flowId}/executions.
func (Unavailable) CreateExecution(context.Context, apigen.CreateExecutionRequestObject) (apigen.CreateExecutionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// PublishFlow refuses POST /flows/{flowId}/publish.
func (Unavailable) PublishFlow(context.Context, apigen.PublishFlowRequestObject) (apigen.PublishFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// RollbackFlow refuses POST /flows/{flowId}/rollback.
func (Unavailable) RollbackFlow(context.Context, apigen.RollbackFlowRequestObject) (apigen.RollbackFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// CreateTestSession refuses POST /flows/{flowId}/test-sessions.
func (Unavailable) CreateTestSession(context.Context, apigen.CreateTestSessionRequestObject) (apigen.CreateTestSessionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// GetTestSession refuses GET /flows/{flowId}/test-sessions/{testSessionId}.
func (Unavailable) GetTestSession(context.Context, apigen.GetTestSessionRequestObject) (apigen.GetTestSessionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// CancelTestSession refuses POST /flows/{flowId}/test-sessions/{testSessionId}/cancel.
func (Unavailable) CancelTestSession(context.Context, apigen.CancelTestSessionRequestObject) (apigen.CancelTestSessionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ListFlowVersions refuses GET /flows/{flowId}/versions.
func (Unavailable) ListFlowVersions(context.Context, apigen.ListFlowVersionsRequestObject) (apigen.ListFlowVersionsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// GetFlowVersion refuses GET /flows/{flowId}/versions/{version}.
func (Unavailable) GetFlowVersion(context.Context, apigen.GetFlowVersionRequestObject) (apigen.GetFlowVersionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// CompleteHumanTask refuses POST /human-tasks/{humanTaskId}/complete.
func (Unavailable) CompleteHumanTask(context.Context, apigen.CompleteHumanTaskRequestObject) (apigen.CompleteHumanTaskResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ListProjects refuses GET /orgs/{orgId}/projects.
func (Unavailable) ListProjects(context.Context, apigen.ListProjectsRequestObject) (apigen.ListProjectsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// CreateProject refuses POST /orgs/{orgId}/projects.
func (Unavailable) CreateProject(context.Context, apigen.CreateProjectRequestObject) (apigen.CreateProjectResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// DeleteProject refuses DELETE /projects/{projectId}.
func (Unavailable) DeleteProject(context.Context, apigen.DeleteProjectRequestObject) (apigen.DeleteProjectResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// GetProject refuses GET /projects/{projectId}.
func (Unavailable) GetProject(context.Context, apigen.GetProjectRequestObject) (apigen.GetProjectResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// UpdateProject refuses PATCH /projects/{projectId}.
func (Unavailable) UpdateProject(context.Context, apigen.UpdateProjectRequestObject) (apigen.UpdateProjectResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ListExecutions refuses GET /projects/{projectId}/executions.
func (Unavailable) ListExecutions(context.Context, apigen.ListExecutionsRequestObject) (apigen.ListExecutionsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ListFlows refuses GET /projects/{projectId}/flows.
func (Unavailable) ListFlows(context.Context, apigen.ListFlowsRequestObject) (apigen.ListFlowsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// CreateFlow refuses POST /projects/{projectId}/flows.
func (Unavailable) CreateFlow(context.Context, apigen.CreateFlowRequestObject) (apigen.CreateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}

// ListNodeTypes refuses GET /projects/{projectId}/node-types.
func (Unavailable) ListNodeTypes(context.Context, apigen.ListNodeTypesRequestObject) (apigen.ListNodeTypesResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return nil, errUnavailable
}
