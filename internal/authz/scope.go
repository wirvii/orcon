// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// OrganizationOf returns the organization scope of a request: the organization context the principal carries
// (ADR-0046 section 2). Only this package issues scopes for requests, so a handler that needs one asks for it here,
// after the evaluator has decided the request. A principal without an organization context has no scope.
func OrganizationOf(p identity.Principal) (orgscope.Org, error) {
	return issue.New(id.UUID(p.OrgID))
}
