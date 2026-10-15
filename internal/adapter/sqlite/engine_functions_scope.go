// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// ScopeOf implements [store.Scopes]: the organization of an engine row becomes the
// scope the organization-scoped methods take. The engine functions are the only
// source of these ids for the engine.
func (s *Store) ScopeOf(org identity.OrgID) (orgscope.Org, error) {
	return issue.New(id.UUID(org))
}
