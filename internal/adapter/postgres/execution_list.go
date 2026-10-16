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
// of the project that match the query, newest first (oldest first under query.OldestFirst), past the
// cursor in that direction, each with its flow version number and whether it has an operation whose
// outcome is unknown.
func (s *Store) ListProjectExecutions(ctx context.Context, org orgscope.Org, project identity.ProjectID, query store.ExecutionQuery, cursor store.Cursor, limit int) ([]store.Execution, error) {
	var out []store.Execution
	err := s.inScope(ctx, "list project executions", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		if query.OldestFirst {
			afterAt, afterID := afterCursor(cursor)
			rows, err := q.ListOrgProjectExecutionsOldest(ctx, sqlcgen.ListOrgProjectExecutionsOldestParams{
				OrgID: pgUUID(org.UUID()), ProjectID: pgUUID(project),
				FlowID: nullPGUUID(query.FlowID), Status: nullText(query.Status),
				ExecutionID:    nullPGUUID(query.ID),
				CreatedAfter:   nullTimestamp(query.CreatedAfter),
				CreatedBefore:  nullTimestamp(query.CreatedBefore),
				UncertainOnly:  query.UncertainOnly,
				AfterCreatedAt: afterAt, AfterID: afterID, PageSize: clampPage(limit),
			})
			if err != nil {
				return err
			}
			for i := range rows {
				converted := sqlcgen.GetExecutionRow(rows[i])
				out = append(out, executionFrom(&converted))
			}
			return nil
		}
		beforeAt, beforeID := beforeCursor(cursor)
		rows, err := q.ListOrgProjectExecutions(ctx, sqlcgen.ListOrgProjectExecutionsParams{
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
