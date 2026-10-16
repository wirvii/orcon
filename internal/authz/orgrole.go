// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// OrganizationRole returns the predefined role the user holds at organization level in org: the
// highest-ranked of its organization bindings (BuiltinRoles order), or "" when it holds none. A
// project binding never counts. The organization menu of the signed-in user shows it (SPEC-019
// R-UX-SHL-03).
func (e *Evaluator) OrganizationRole(ctx context.Context, user identity.UserID, org orgscope.Org) (string, error) {
	if user.IsZero() {
		return "", nil
	}
	bindings, err := e.store.Bindings(ctx, user, org)
	if err != nil {
		return "", fmt.Errorf("organization role: %w", err)
	}
	for _, role := range BuiltinRoles {
		for _, b := range bindings {
			if b.Project == "" && b.Role == role {
				return role, nil
			}
		}
	}
	return "", nil
}
