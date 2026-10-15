// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/wirvii/orcon/internal/adapter/codec/permissionsyaml"
	"github.com/wirvii/orcon/internal/adapter/httpapi"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/config"
)

// newAPIRouter builds the public API of the roles that serve it (api and
// standalone): the contract's operations through the request pipeline, and the
// reference site at /api/reference.
//
// Until the sign-in cut of SPEC-004 provides the real authentication chain and
// its authorization cut provides the evaluator, the process wires the
// implementations that refuse: every protected route answers 401 and nothing is
// ever allowed (identity.RejectAll, authz.DenyAll).
// The public trigger addresses under /hooks/ are served by webhooks; nil serves none.
func newAPIRouter(role config.Role, logger *slog.Logger, webhooks httpapi.WebhookIngress) (*httpapi.Router, error) {
	catalog, err := permissionsyaml.Parse(authz.CatalogSource())
	if err != nil {
		return nil, fmt.Errorf("load the permission catalog: %w", err)
	}
	router, err := httpapi.New(&httpapi.Deps{
		Role:          role,
		Logger:        logger,
		Catalog:       catalog,
		Authenticator: identity.RejectAll{},
		Authorizer:    authz.DenyAll{},
		Webhooks:      webhooks,
	})
	if err != nil {
		return nil, fmt.Errorf("build the API: %w", err)
	}
	return router, nil
}

// mountAPI registers the public API on mux under /api/.
func mountAPI(mux *http.ServeMux, role config.Role, logger *slog.Logger) error {
	return mountAPIWith(mux, role, logger, nil)
}

// mountAPIWith is mountAPI that also serves the public trigger addresses under
// /hooks/ through the same router, so they share its request pipeline.
func mountAPIWith(mux *http.ServeMux, role config.Role, logger *slog.Logger, webhooks httpapi.WebhookIngress) error {
	router, err := newAPIRouter(role, logger, webhooks)
	if err != nil {
		return err
	}
	mux.Handle("/api/", router)
	if webhooks != nil {
		mux.Handle(httpapi.WebhookPrefix, router)
	}
	return nil
}
