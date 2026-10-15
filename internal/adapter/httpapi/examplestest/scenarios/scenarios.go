// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package scenarios holds the scenarios the x-orcon-examples of the API
// contract name (orcon-api DOC-9). A scenario seeds the state one example needs
// through the public API only, never through storage, and never reaches the
// internet. It returns the bindings from the fixed ids of the examples to the
// ids it created.
//
// The scenarios are plain functions collected by [All]; a test or the examples
// runner registers them with examplestest.RegisterScenario or hands them to the
// runner as its scenario table. Nothing registers itself on import.
package scenarios

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/adapter/httpapi/examplestest"
)

// The fixed identifiers the examples of the contract use. They are constants of
// the contract, bound to the real ones by each scenario.
const (
	fixedOrg      = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a71"
	fixedProject  = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a70"
	fixedProject2 = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a77"
	fixedFlow     = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60"
	fixedFlow2    = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a63"
	fixedExec     = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a64"
	fixedNodeExec = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a66"
	fixedSession  = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a69"
)

// awaitingSignIn returns the scenario of an example that needs a signed-in user.
// Signing in is not connected yet (SPEC-016 SH2 connects the session operations
// and SH5 the local account commands), so the scenario says so instead of seeding
// state through a path that does not exist.
func awaitingSignIn(name string) examplestest.Scenario {
	return func(context.Context, *examplestest.Client) (examplestest.Bindings, error) {
		return nil, fmt.Errorf("scenario %s: %w: it needs a signed-in user; the sign-in operations are not connected yet (SPEC-016 SH2, SH5)", name, ErrNotSeedable)
	}
}

// All returns every scenario by name. The names are the ones the contract uses.
func All() map[string]examplestest.Scenario {
	return map[string]examplestest.Scenario{
		"account-with-password":              awaitingSignIn("account-with-password"),
		"signed-in-user":                     awaitingSignIn("signed-in-user"),
		"user-with-organizations":            awaitingSignIn("user-with-organizations"),
		"organization-ready":                 organizationReady,
		"organization-with-projects":         organizationWithProjects,
		"empty-project":                      emptyProject,
		"project-with-flows":                 projectWithFlows,
		"flow-with-valid-draft":              flowWithValidDraft,
		"flow-published":                     flowPublished,
		"flow-with-two-versions":             flowWithTwoVersions,
		"flow-active":                        flowActive,
		"flow-with-test-session":             flowWithTestSession,
		"execution-succeeded":                executionSucceeded,
		"execution-waiting":                  executionWaiting,
		"execution-failed":                   executionFailed,
		"execution-with-human-task":          executionWithHumanTask,
		"execution-with-uncertain-operation": executionWithUncertainOperation,
	}
}
