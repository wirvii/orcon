// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/wirvii/orcon/internal/adapter/codec/permissionsyaml"
	"github.com/wirvii/orcon/internal/adapter/httpapi"
	"github.com/wirvii/orcon/internal/adapter/httpapi/idempotency"
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
func newAPIRouter(role config.Role, logger *slog.Logger) (*httpapi.Router, error) {
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
		// No durable store exists yet; operations that take an Idempotency-Key answer 503
		// until the first API2-core pull request wires it.
		Idempotency: idempotency.UnavailableStore{},
	})
	if err != nil {
		return nil, fmt.Errorf("build the API: %w", err)
	}
	return router, nil
}

// mountAPI registers the public API on mux under /api/.
func mountAPI(mux *http.ServeMux, role config.Role, logger *slog.Logger) error {
	router, err := newAPIRouter(role, logger)
	if err != nil {
		return err
	}
	mux.Handle("/api/", router)
	return nil
}
