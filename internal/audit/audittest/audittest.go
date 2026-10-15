// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package audittest builds valid audit entries for the tests of the storage
// adapters and of the areas that write them.
package audittest

import (
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/platform/id"
)

// Epoch is the instant the test entries carry unless a test sets another one.
var Epoch = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// Org returns a successful entry of the organization chain of org, by an actor
// that is a user principal, for the action.
func Org(tb testing.TB, org id.UUID, action string) audit.Entry {
	tb.Helper()
	return build(tb, audit.OrganizationChain(org), action)
}

// Instance returns a successful entry of the instance chain for the action, by an
// anonymous actor.
func Instance(tb testing.TB, action string) audit.Entry {
	tb.Helper()
	return build(tb, audit.InstanceChain(), action)
}

// InstanceBy returns a successful entry of the instance chain for the action, by a user principal.
func InstanceBy(tb testing.TB, principal id.UUID, action string) audit.Entry {
	tb.Helper()
	e, err := audit.NewEntry(audit.EntryParams{
		ID: newID(), Chain: audit.InstanceChain(), OccurredAt: Epoch,
		Actor: audit.Actor{PrincipalID: principal, Kind: audit.ActorUser}, Action: action, Outcome: audit.OutcomeSuccess,
	})
	if err != nil {
		tb.Fatalf("audittest: %v", err)
	}
	return e
}

func build(tb testing.TB, chain audit.Chain, action string) audit.Entry {
	tb.Helper()
	e, err := audit.NewEntry(audit.EntryParams{
		ID: newID(), Chain: chain, OccurredAt: Epoch, Action: action, Outcome: audit.OutcomeSuccess,
	})
	if err != nil {
		tb.Fatalf("audittest: %v", err)
	}
	return e
}

// newID returns a fresh UUIDv7; each call has its own generator, so parallel tests never share identities.
func newID() id.UUID { return id.NewV7Generator(time.Now).New() }
