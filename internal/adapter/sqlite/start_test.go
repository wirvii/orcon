// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// TestSingleProcessLockReleasesLeases: the process lock guarantees that no
// other process holds a lease, so a start releases every token a crashed
// predecessor left LEASED (ADR-0013 section 9, E-STO-3).
func TestSingleProcessLockReleasesLeases(t *testing.T) {
	path := dbPath(t)
	s := open(t, path)
	ctx := t.Context()
	ids := id.NewSequence()
	now := time.Now().UTC()
	org := identity.OrgID(ids.New())
	scope, err := issue.New(id.UUID(org))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOrganization(ctx, identity.OrgRecord{ID: org, Name: "A", Slug: "a", Status: identity.OrgActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	project := identity.ProjectID(ids.New())
	if err := s.CreateProject(ctx, scope, identity.ProjectRecord{ID: project, OrgID: org, Name: "P", Slug: "p", IsProtected: true, CreatedAt: now, UpdatedAt: now}, audittest.Org(t, id.UUID(org), "project.created")); err != nil {
		t.Fatal(err)
	}
	rec := flow.Record{ID: flow.FlowID(ids.New()), OrgID: org, ProjectID: project, Name: "f", Draft: []byte(`{}`), CreatedAt: now, UpdatedAt: now}
	if err := s.CreateFlow(ctx, scope, rec); err != nil {
		t.Fatal(err)
	}
	ver := flow.VersionRecord{ID: flow.FlowVersionID(ids.New()), OrgID: org, FlowID: rec.ID, Number: 1, Document: []byte(`{}`), ContentHash: "h", PublishedAt: now, CreatedAt: now}
	if err := s.PublishVersion(ctx, scope, ver); err != nil {
		t.Fatal(err)
	}
	ex := store.Execution{ID: store.ExecutionID(ids.New()), OrgID: org, ProjectID: project, FlowID: rec.ID, FlowVersionID: ver.ID, Status: "RUNNING", Mode: "MANUAL", CreatedAt: now, UpdatedAt: now}
	err = s.InTx(ctx, scope, func(ctx context.Context, tx store.Tx) error {
		if err := tx.InsertExecution(ctx, &ex); err != nil {
			return err
		}
		for range 3 {
			tok := store.Token{ID: store.TokenID(ids.New()), OrgID: org, ExecutionID: ex.ID, Status: "RUNNABLE", NodeID: "n", ForkLineage: []byte(`[]`),
				FairnessKey: "k", AvailableAt: now.Add(-time.Hour), CreatedAt: now, UpdatedAt: now}
			if err := tx.InsertToken(ctx, &tok); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	worker := execution.WorkerID(ids.New())
	claimed, err := s.ClaimTokens(ctx, store.ClaimRequest{Worker: worker, Limit: 2, LeaseTTL: time.Hour, Now: now})
	if err != nil || len(claimed) != 2 {
		t.Fatalf("claim = %d, %v", len(claimed), err)
	}
	// The process "crashes": the file is closed with two leases held.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := open(t, path)
	for _, c := range claimed {
		got, err := restarted.GetToken(ctx, scope, c.TokenID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != "RUNNABLE" || !got.LeaseOwner.IsZero() || !got.LeaseExpiresAt.IsZero() {
			t.Fatalf("token %s after the restart = %+v", c.TokenID, got)
		}
		if got.LeaseEpoch != 1 {
			t.Fatalf("the epoch must survive the release so that the next claim raises it: %d", got.LeaseEpoch)
		}
	}
	// And they are claimable again, with a higher epoch.
	again, err := restarted.ClaimTokens(ctx, store.ClaimRequest{Worker: worker, Limit: 10, LeaseTTL: time.Hour, Now: time.Now().UTC()})
	if err != nil || len(again) != 3 {
		t.Fatalf("claim after the restart = %d, %v", len(again), err)
	}
	for _, c := range again {
		for _, old := range claimed {
			if c.TokenID == old.TokenID && c.LeaseEpoch != 2 {
				t.Fatalf("token %s reclaimed at epoch %d, want 2", c.TokenID, c.LeaseEpoch)
			}
		}
	}
}
