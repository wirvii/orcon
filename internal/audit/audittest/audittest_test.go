// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package audittest_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/platform/id"
)

// TestBuildersReturnValidEntries: every builder returns an entry the audit package accepts,
// with the chain, actor and outcome its name promises, at the shared epoch.
func TestBuildersReturnValidEntries(t *testing.T) {
	org := id.UUID{15: 3}
	principal := id.UUID{15: 4}

	o := audittest.Org(t, org, "project.created")
	if o.Chain.Org() != org || o.Chain.IsInstance() || o.Action != "project.created" {
		t.Errorf("Org: chain %v action %q, want the organization chain of %v", o.Chain, o.Action, org)
	}

	i := audittest.Instance(t, "login.succeeded")
	if !i.Chain.IsInstance() || i.Action != "login.succeeded" || !i.Actor.IsAnonymous() {
		t.Errorf("Instance: %+v, want an anonymous entry of the instance chain", i)
	}

	by := audittest.InstanceBy(t, principal, "account.created")
	if !by.Chain.IsInstance() || by.Actor.PrincipalID != principal || by.Actor.Kind != audit.ActorUser {
		t.Errorf("InstanceBy: actor %+v, want the user principal %v", by.Actor, principal)
	}
	for _, e := range []audit.Entry{o, i, by} {
		if e.Outcome != audit.OutcomeSuccess || !e.OccurredAt.Equal(audittest.Epoch) || e.ID.IsZero() {
			t.Errorf("entry %q: outcome %q at %v with id %v, want success at the epoch with an id", e.Action, e.Outcome, e.OccurredAt, e.ID)
		}
	}
	if o.ID == i.ID || i.ID == by.ID {
		t.Error("two builders returned the same identifier")
	}
}
