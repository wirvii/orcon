// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// ListProjectExecutions implements [store.ProjectExecutionReader] (SPEC-019 R-UX-BE-03): the executions
// of the project that match the query, newest first, after the cursor, each with its flow version number
// and whether it has an operation whose outcome is unknown.
func (s *Store) ListProjectExecutions(ctx context.Context, org orgscope.Org, project identity.ProjectID, query store.ExecutionQuery, after store.Cursor, limit int) ([]store.Execution, error) {
	beforeAt, beforeID := beforeCursor(after)
	var out []store.Execution
	err := s.inScope(ctx, "list project executions", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListProjectExecutions(ctx, sqlcgen.ListProjectExecutionsParams{
			OrgID: pgUUID(org.UUID()), ProjectID: pgUUID(project),
			FlowID: nullPGUUID(query.FlowID), Status: nullText(query.Status),
			ExecutionID:     nullPGUUID(query.ID),
			CreatedAfter:    nullTimestamp(query.CreatedAfter),
			CreatedBefore:   nullTimestamp(query.CreatedBefore),
			UncertainOnly:   query.UncertainOnly,
			BeforeCreatedAt: beforeAt, BeforeID: beforeID, PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetExecutionRow(rows[i])
			out = append(out, executionFrom(&converted))
		}
		return nil
	})
	return out, err
}
