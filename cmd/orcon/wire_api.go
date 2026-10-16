// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/wirvii/orcon/internal/adapter/codec/permissionsyaml"
	"github.com/wirvii/orcon/internal/adapter/httpapi"
	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/idempotency"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/config"
)

// newAPIRouterWith builds the public API of the roles that serve it (api and
// standalone): the contract's operations through the request pipeline, and the
// reference site at /api/reference. A nil sec keeps
// the refusing implementations (identity.RejectAll, authz.DenyAll): every protected route
// answers 401 and nothing is allowed. With sec, the sessions authenticate the
// requests, the evaluator decides them, and the operations that are wired answer from the
// services; the others keep answering 503.
//
// The public trigger addresses under /hooks/ are served by webhooks; nil serves none.
func newAPIRouterWith(role config.Role, logger *slog.Logger, sec *security, webhooks httpapi.WebhookIngress) (*httpapi.Router, error) {
	catalog, err := permissionsyaml.Parse(authz.CatalogSource())
	if err != nil {
		return nil, fmt.Errorf("load the permission catalog: %w", err)
	}
	deps := &httpapi.Deps{
		Role:          role,
		Logger:        logger,
		Catalog:       catalog,
		Authenticator: identity.RejectAll{},
		Authorizer:    authz.DenyAll{},
		Webhooks:      webhooks,
		// Without the security wiring there is no store, so an operation that takes an
		// Idempotency-Key answers 503 rather than running without the protection.
		Idempotency: idempotency.UnavailableStore{},
	}
	if sec != nil {
		assets, err := officialConnectorAssets()
		if err != nil {
			return nil, err
		}
		deps.Authenticator = sec.sessions
		deps.Authorizer = sec.evaluator
		deps.OrgSelector = sec.evaluator
		deps.Idempotency = sec.store
		projects := httpapi.NewProjects(
			sec.evaluator, identity.NewProjects(sec.store, sec.store, sec.clock, sec.ids), sec.store,
			flow.NewCreator(sec.store, sec.clock, sec.ids), sec.store, sec.pageCodec, sec.clock,
		)
		deps.Rebuilds = projects.Rebuilds()
		flowDrafts := flow.NewDrafts(sec.store, sec.clock, sec.ids)
		drafts := httpapi.NewDrafts(flowDrafts)
		// The trigger service of the engine runs the lifecycle; a process without an engine has
		// none, and those operations keep answering 503.
		lifecycle, _ := webhooks.(httpapi.FlowLifecycle)
		flows := httpapi.NewFlows(flow.NewManager(sec.store, sec.clock, sec.ids), lifecycle, sec.store, flowDrafts, sec.pageCodec)
		deps.Server = productServer{
			StrictServerInterface: httpapi.Unavailable{},
			sessions:              httpapi.NewSessions(sec.sessions, middleware.DefaultAuthConfig(), sec.publicOrigin, sec.store, sec.evaluator),
			orgs:                  httpapi.NewOrgs(sec.store),
			projects:              projects,
			nodeTypes:             httpapi.NewNodeTypes(httpapi.NewNodeCatalog(sec.store, assets), sec.pageCodec),
			drafts:                drafts,
			flows:                 flows,
			executions: httpapi.NewExecutions(
				sec.store, sec.store, flowReader{}, sec.store, sec.evaluator, sec.store, sec.clock, sec.ids, sec.pageCodec,
			),
			activity:      httpapi.NewActivity(sec.evaluator, sec.store, sec.store, sec.clock),
			orgExecutions: httpapi.NewOrgExecutions(sec.evaluator, sec.store, sec.store, sec.pageCodec),
			credentials:   httpapi.NewCredentials(sec.credentials),
		}
	}
	router, err := httpapi.New(deps)
	if err != nil {
		return nil, fmt.Errorf("build the API: %w", err)
	}
	return router, nil
}

// productServer answers the operations that the security wiring serves; every other
// operation of the contract falls through to base and answers 503.
type productServer struct {
	apigen.StrictServerInterface
	sessions *httpapi.Sessions
	orgs     *httpapi.Orgs
	projects *httpapi.Projects
	// nodeTypes serves the node types a project can use.
	nodeTypes *httpapi.NodeTypes
	drafts    *httpapi.Drafts
	// executions serves the read side of the executions.
	executions *httpapi.Executions
	activity   *httpapi.Activity
	// orgExecutions serves the executions of an organization.
	orgExecutions *httpapi.OrgExecutions
	flows         *httpapi.Flows
	// credentials serves the credential operations.
	credentials *httpapi.Credentials
}

func (s productServer) ListProjects(ctx context.Context, req apigen.ListProjectsRequestObject) (apigen.ListProjectsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.projects.ListProjects(ctx, req)
}

func (s productServer) CreateProject(ctx context.Context, req apigen.CreateProjectRequestObject) (apigen.CreateProjectResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.projects.CreateProject(ctx, req)
}

func (s productServer) ListFlows(ctx context.Context, req apigen.ListFlowsRequestObject) (apigen.ListFlowsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.projects.ListFlows(ctx, req)
}

func (s productServer) CreateFlow(ctx context.Context, req apigen.CreateFlowRequestObject) (apigen.CreateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.projects.CreateFlow(ctx, req)
}

func (s productServer) ListNodeTypes(ctx context.Context, req apigen.ListNodeTypesRequestObject) (apigen.ListNodeTypesResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.nodeTypes.ListNodeTypes(ctx, req)
}

func (s productServer) GetDraft(ctx context.Context, req apigen.GetDraftRequestObject) (apigen.GetDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.drafts.GetDraft(ctx, req)
}

func (s productServer) ReplaceDraft(ctx context.Context, req apigen.ReplaceDraftRequestObject) (apigen.ReplaceDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.drafts.ReplaceDraft(ctx, req)
}

func (s productServer) ListExecutions(ctx context.Context, req apigen.ListExecutionsRequestObject) (apigen.ListExecutionsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.executions.ListExecutions(ctx, req)
}

func (s productServer) GetExecution(ctx context.Context, req apigen.GetExecutionRequestObject) (apigen.GetExecutionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.executions.GetExecution(ctx, req)
}

func (s productServer) ListExecutionNodes(ctx context.Context, req apigen.ListExecutionNodesRequestObject) (apigen.ListExecutionNodesResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.executions.ListExecutionNodes(ctx, req)
}

func (s productServer) ListExecutionData(ctx context.Context, req apigen.ListExecutionDataRequestObject) (apigen.ListExecutionDataResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.executions.ListExecutionData(ctx, req)
}

func (s productServer) ListExecutionEvents(ctx context.Context, req apigen.ListExecutionEventsRequestObject) (apigen.ListExecutionEventsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.executions.ListExecutionEvents(ctx, req)
}

func (s productServer) GetFlow(ctx context.Context, req apigen.GetFlowRequestObject) (apigen.GetFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.flows.GetFlow(ctx, req)
}

func (s productServer) UpdateFlow(ctx context.Context, req apigen.UpdateFlowRequestObject) (apigen.UpdateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.flows.UpdateFlow(ctx, req)
}

func (s productServer) DeleteFlow(ctx context.Context, req apigen.DeleteFlowRequestObject) (apigen.DeleteFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.flows.DeleteFlow(ctx, req)
}

func (s productServer) PublishFlow(ctx context.Context, req apigen.PublishFlowRequestObject) (apigen.PublishFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.flows.PublishFlow(ctx, req)
}

func (s productServer) ActivateFlow(ctx context.Context, req apigen.ActivateFlowRequestObject) (apigen.ActivateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.flows.ActivateFlow(ctx, req)
}

func (s productServer) DeactivateFlow(ctx context.Context, req apigen.DeactivateFlowRequestObject) (apigen.DeactivateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.flows.DeactivateFlow(ctx, req)
}

func (s productServer) RollbackFlow(ctx context.Context, req apigen.RollbackFlowRequestObject) (apigen.RollbackFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.flows.RollbackFlow(ctx, req)
}

func (s productServer) ListFlowVersions(ctx context.Context, req apigen.ListFlowVersionsRequestObject) (apigen.ListFlowVersionsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.flows.ListFlowVersions(ctx, req)
}

func (s productServer) GetFlowVersion(ctx context.Context, req apigen.GetFlowVersionRequestObject) (apigen.GetFlowVersionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.flows.GetFlowVersion(ctx, req)
}

func (s productServer) ValidateDraft(ctx context.Context, req apigen.ValidateDraftRequestObject) (apigen.ValidateDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.flows.ValidateDraft(ctx, req)
}

func (s productServer) UpdateDraft(ctx context.Context, req apigen.UpdateDraftRequestObject) (apigen.UpdateDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.drafts.UpdateDraft(ctx, req)
}

func (s productServer) ListCredentials(ctx context.Context, req apigen.ListCredentialsRequestObject) (apigen.ListCredentialsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.credentials.ListCredentials(ctx, req)
}

func (s productServer) CreateCredential(ctx context.Context, req apigen.CreateCredentialRequestObject) (apigen.CreateCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.credentials.CreateCredential(ctx, req)
}

func (s productServer) GetCredential(ctx context.Context, req apigen.GetCredentialRequestObject) (apigen.GetCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.credentials.GetCredential(ctx, req)
}

func (s productServer) UpdateCredential(ctx context.Context, req apigen.UpdateCredentialRequestObject) (apigen.UpdateCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.credentials.UpdateCredential(ctx, req)
}

func (s productServer) ReplaceCredentialSecret(ctx context.Context, req apigen.ReplaceCredentialSecretRequestObject) (apigen.ReplaceCredentialSecretResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.credentials.ReplaceCredentialSecret(ctx, req)
}

func (s productServer) DeleteCredential(ctx context.Context, req apigen.DeleteCredentialRequestObject) (apigen.DeleteCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.credentials.DeleteCredential(ctx, req)
}

func (s productServer) CreateCredentialShare(ctx context.Context, req apigen.CreateCredentialShareRequestObject) (apigen.CreateCredentialShareResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.credentials.CreateCredentialShare(ctx, req)
}

func (s productServer) DeleteCredentialShare(ctx context.Context, req apigen.DeleteCredentialShareRequestObject) (apigen.DeleteCredentialShareResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.credentials.DeleteCredentialShare(ctx, req)
}

func (s productServer) CreateSession(ctx context.Context, req apigen.CreateSessionRequestObject) (apigen.CreateSessionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.sessions.CreateSession(ctx, req)
}

func (s productServer) RevokeSession(ctx context.Context, req apigen.RevokeSessionRequestObject) (apigen.RevokeSessionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.sessions.RevokeSession(ctx, req)
}

func (s productServer) GetCurrentUser(ctx context.Context, req apigen.GetCurrentUserRequestObject) (apigen.GetCurrentUserResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.sessions.GetCurrentUser(ctx, req)
}

func (s productServer) ListOrgFlows(ctx context.Context, req apigen.ListOrgFlowsRequestObject) (apigen.ListOrgFlowsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.projects.ListOrgFlows(ctx, req)
}

func (s productServer) ListOrgExecutions(ctx context.Context, req apigen.ListOrgExecutionsRequestObject) (apigen.ListOrgExecutionsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.orgExecutions.ListOrgExecutions(ctx, req)
}

func (s productServer) GetOrgActivitySummary(ctx context.Context, req apigen.GetOrgActivitySummaryRequestObject) (apigen.GetOrgActivitySummaryResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.activity.GetOrgActivitySummary(ctx, req)
}

func (s productServer) ListOrgs(ctx context.Context, req apigen.ListOrgsRequestObject) (apigen.ListOrgsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.orgs.ListOrgs(ctx, req)
}

// mountAPI registers the public API on mux under /api/.
func mountAPI(mux *http.ServeMux, role config.Role, logger *slog.Logger) error {
	return mountAPIWith(mux, role, logger, nil, nil)
}

// mountAPIWith registers the API over the security wiring on mux under /api/. The public
// trigger addresses under /hooks/ go through the same router when webhooks is not nil, so they
// share its request pipeline.
func mountAPIWith(mux *http.ServeMux, role config.Role, logger *slog.Logger, sec *security, webhooks httpapi.WebhookIngress) error {
	router, err := newAPIRouterWith(role, logger, sec, webhooks)
	if err != nil {
		return err
	}
	mux.Handle("/api/", router)
	if webhooks != nil {
		mux.Handle(httpapi.WebhookPrefix, router)
	}
	return nil
}
