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
		deps.Server = productServer{
			StrictServerInterface: httpapi.Unavailable{},
			sessions:              httpapi.NewSessions(sec.sessions, middleware.DefaultAuthConfig(), sec.publicOrigin, sec.store, sec.evaluator),
			orgs:                  httpapi.NewOrgs(sec.store),
			projects:              projects,
			nodeTypes:             httpapi.NewNodeTypes(httpapi.NewNodeCatalog(sec.store, assets), sec.pageCodec),
			activity:              httpapi.NewActivity(sec.evaluator, sec.store, sec.store, sec.clock),
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
	activity  *httpapi.Activity
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
