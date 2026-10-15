// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

// The in-memory store answers the reads the battery does not make itself. An unknown
// identity is refused, and no read invents a record.
func TestMemoryReadsRefuseUnknownIdentities(t *testing.T) {
	m := triggertest.NewMemory()
	org := scopeFor(identity.OrgID{1})
	ctx := t.Context()

	if _, err := m.GetOrganization(ctx, org); err == nil {
		t.Error("an unknown organization was read")
	}
	if _, err := m.GetFlowByName(ctx, org, identity.ProjectID{2}, "flow"); err == nil {
		t.Error("an unknown flow name was read")
	}
	if _, err := m.GetSnapshot(ctx, org, flow.FlowSnapshotID{3}); err == nil {
		t.Error("an unknown snapshot was read")
	}
	if _, err := m.ResolveRoute(ctx, "POST", "/hooks/none"); err == nil {
		t.Error("an unknown webhook route was resolved")
	}
	if projects, err := m.ListProjects(ctx, org, identity.ProjectID{}, 10); err != nil || len(projects) != 0 {
		t.Errorf("projects of an empty organization = %d, %v", len(projects), err)
	}
	if flows, err := m.ListFlows(ctx, org, identity.ProjectID{}, flow.FlowID{}, 10); err != nil || len(flows) != 0 {
		t.Errorf("flows of an empty project = %d, %v", len(flows), err)
	}
	if _, err := m.GetSystemPrincipal(ctx); err != nil {
		t.Logf("the system principal is not seeded: %v", err)
	}
}
