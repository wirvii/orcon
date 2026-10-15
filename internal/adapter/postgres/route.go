// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/trigger"
)

// ResolveRoute implements [trigger.Store]: the engine plane reads the one
// address that answers the request, without an organization context (migration
// 0023, function orcon_engine.resolve_trigger_route).
func (s *Store) ResolveRoute(ctx context.Context, method, path string) (trigger.Resolved, error) {
	var out trigger.Resolved
	found := false
	err := s.engineRows(ctx, "resolve route", `SELECT org_id, project_id, flow_id, node_id, flow_trigger_id, trigger_address_id FROM orcon_engine.resolve_trigger_route($1, $2)`,
		[]any{method, path},
		func(rows pgx.Rows) error {
			var org, project, flowID, ft, addr pgtype.UUID
			var node string
			if err := rows.Scan(&org, &project, &flowID, &node, &ft, &addr); err != nil {
				return err
			}
			out = trigger.Resolved{
				Org:       fromPGUUID[identity.OrgID](org),
				ProjectID: fromPGUUID[identity.ProjectID](project),
				FlowID:    fromPGUUID[flow.FlowID](flowID),
				NodeID:    flow.NodeID(node),
				TriggerID: fromPGUUID[trigger.ID](ft),
				AddressID: fromPGUUID[trigger.AddressID](addr),
			}
			found = true
			return nil
		})
	if err != nil {
		return trigger.Resolved{}, fmt.Errorf("resolve route: %w", err)
	}
	if !found {
		return trigger.Resolved{}, trigger.ErrNoRoute
	}
	scope, err := s.ScopeOf(out.Org)
	if err != nil {
		return trigger.Resolved{}, fmt.Errorf("resolve route: %w", err)
	}
	out.Scope = scope
	return out, nil
}
