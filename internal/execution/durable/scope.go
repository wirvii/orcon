// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// engineScope turns the organization of a row the engine read into the scope the
// organization-scoped methods of the store take (ADR-0046 section 2, ADR-0047
// section 2). The store issues it: the engine holds only ids from its own rows.
func engineScope(s store.Scopes, org identity.OrgID) (orgscope.Org, error) {
	o, err := s.ScopeOf(org)
	if err != nil {
		return orgscope.Org{}, fmt.Errorf("organization scope: %w", err)
	}
	return o, nil
}

// inTx runs fn in a transaction of the organization of an engine row.
func (e *Engine) inTx(ctx context.Context, org identity.OrgID, fn func(ctx context.Context, tx store.Tx) error) error {
	scope, err := engineScope(e.store, org)
	if err != nil {
		return err
	}
	return e.store.InTx(ctx, scope, fn)
}
