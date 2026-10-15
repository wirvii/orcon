// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

var (
	_ identity.OrgStore        = (*Store)(nil)
	_ identity.PrincipalStore  = (*Store)(nil)
	_ identity.UserStore       = (*Store)(nil)
	_ identity.MembershipStore = (*Store)(nil)
	_ identity.ProjectStore    = (*Store)(nil)
	_ flow.Store               = (*Store)(nil)
	_ flow.VersionStore        = (*Store)(nil)
	_ flow.SnapshotStore       = (*Store)(nil)
	_ execution.WorkerStore    = (*Store)(nil)
)

// RegisterWorker implements [execution.WorkerStore].
func (s *Store) RegisterWorker(ctx context.Context, w execution.WorkerRecord) error {
	now := s.clock.Now()
	return s.write(ctx, "register worker", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpsertWorker(ctx, sqlcgen.UpsertWorkerParams{
			ID: uuidBytes(w.ID), Role: w.Role, Host: w.Host, Version: w.Version,
			StartedAt: micros(w.StartedAt), HeartbeatAt: micros(w.HeartbeatAt),
			CreatedAt: micros(now), UpdatedAt: micros(now),
		})
	})
}

// maxPage is the largest page a store returns (orcon-db QY-6).
const maxPage = 1000

func clampPage(limit int) int64 {
	switch {
	case limit < 1:
		return 1
	case limit > maxPage:
		return maxPage
	default:
		return int64(limit)
	}
}
