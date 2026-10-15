// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest_test

import (
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// scopeIssuer issues the scopes of the tests (the store does that in production).
type scopeIssuer struct{}

func (scopeIssuer) ScopeOf(org identity.OrgID) (orgscope.Org, error) {
	return scopeFor(org), nil
}

// scopeFor is the scope of an organization of a scenario; only tests create scopes.
func scopeFor(org identity.OrgID) orgscope.Org {
	scope, err := issue.New(id.UUID(org))
	if err != nil {
		panic(err)
	}
	return scope
}

// memoryBackend is the in-memory store with the scopes of the scenarios.
func memoryBackend(*testing.T) triggertest.Backend {
	m := triggertest.NewMemory()
	m.Scopes = scopeIssuer{}
	b := m.Backend()
	b.Scope = scopeFor
	return b
}

// TestMemoryPassesTheBattery keeps the in-memory store honest: it must give
// the answers the SQLite and PostgreSQL adapters give.
func TestMemoryPassesTheBattery(t *testing.T) {
	t.Parallel()
	triggertest.Battery(t, memoryBackend)
}

func TestMemoryHotSwitch(t *testing.T) {
	t.Parallel()
	load := triggertest.Load{Rate: 400, Duration: 400 * time.Millisecond}
	triggertest.HotSwitchUnderLoad(t, memoryBackend, load)
}
