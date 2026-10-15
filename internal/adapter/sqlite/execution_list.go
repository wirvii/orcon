// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
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
	err := s.readOf(ctx, "list project executions", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListProjectExecutions(ctx, sqlcgen.ListProjectExecutionsParams{
			OrgID: uuidBytes(org.UUID()), ProjectID: uuidBytes(project),
			FlowID: nullUUIDBytes(query.FlowID), Status: sql.NullString{String: query.Status, Valid: query.Status != ""},
			ExecutionID:     nullUUIDBytes(query.ID),
			CreatedAfter:    nullMicros(query.CreatedAfter),
			CreatedBefore:   nullMicros(query.CreatedBefore),
			UncertainOnly:   boolInt(query.UncertainOnly),
			BeforeCreatedAt: beforeAt, BeforeID: beforeID, PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetExecutionRow(rows[i])
			e, err := executionFrom(&converted)
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return nil
	})
	return out, err
}
