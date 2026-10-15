// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
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
	return s.instance(ctx, "register worker", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpsertWorker(ctx, sqlcgen.UpsertWorkerParams{
			ID: pgUUID(w.ID), Role: w.Role, Host: w.Host, Version: w.Version,
			StartedAt: timestamp(w.StartedAt), HeartbeatAt: timestamp(w.HeartbeatAt),
			CreatedAt: timestamp(now), UpdatedAt: timestamp(now),
		})
	})
}
