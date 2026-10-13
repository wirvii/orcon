// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package scenarios

import (
	"context"
	"fmt"
	"net/http"

	"github.com/wirvii/orcon/internal/adapter/httpapi/examplestest"
)

const (
	projectName = "Customer onboarding"
	flowName    = "Welcome email"
)

// organizationReady seeds nothing: the organization of the credential exists.
func organizationReady(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	return newSeeder(ctx, api).result()
}

// organizationWithProjects seeds two projects.
func organizationWithProjects(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	s := newSeeder(ctx, api)
	s.project(projectName, "onboarding", fixedProject)
	s.project("Internal tools", "internal-tools", fixedProject2)
	return s.result()
}

// emptyProject seeds one project with no flow.
func emptyProject(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	s := newSeeder(ctx, api)
	s.project(projectName, "onboarding", fixedProject)
	return s.result()
}

// projectWithFlows seeds a project with two flows that have empty drafts.
func projectWithFlows(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	s := newSeeder(ctx, api)
	p := s.project(projectName, "onboarding", fixedProject)
	s.flow(p, flowName, fixedFlow)
	s.flow(p, "Weekly report", fixedFlow2)
	return s.result()
}

// validFlow seeds a project and a flow whose draft is the valid greeting flow, at revision 1.
func validFlow(s *seeder) string {
	p := s.project(projectName, "onboarding", fixedProject)
	f := s.flow(p, flowName, fixedFlow)
	s.saveDraft(f, 0, greetingDocument(flowName))
	return f
}

func flowWithValidDraft(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	s := newSeeder(ctx, api)
	validFlow(s)
	return s.result()
}

// flowPublished seeds a flow with one published version and no release note.
func flowPublished(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	s := newSeeder(ctx, api)
	f := validFlow(s)
	s.publish(f, 1)
	return s.result()
}

// flowWithTwoVersions seeds a flow with versions 1 and 2.
func flowWithTwoVersions(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	s := newSeeder(ctx, api)
	f := validFlow(s)
	s.publish(f, 1)
	s.saveDraft(f, 1, greetingDocument("Welcome email v2"))
	s.publish(f, 2)
	return s.result()
}

// flowActive seeds a published, active flow.
func flowActive(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	s := newSeeder(ctx, api)
	f := validFlow(s)
	s.publish(f, 1)
	s.activate(f)
	return s.result()
}

// flowWithTestSession seeds a flow with an active test session.
func flowWithTestSession(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	s := newSeeder(ctx, api)
	f := validFlow(s)
	var session struct {
		ID string `json:"id"`
	}
	s.call(examplestest.Request{
		Method: http.MethodPost, Path: "/flows/" + f + "/test-sessions", Header: key("seed-session"),
		Body: map[string]any{"expiresInSeconds": 120, "maximumEventCount": 1},
	}, http.StatusCreated, &session)
	s.bind(fixedSession, session.ID)
	return s.result()
}

// activeFlowOf seeds a project and an active flow whose document is document.
func activeFlowOf(s *seeder, document map[string]any) string {
	p := s.project(projectName, "onboarding", fixedProject)
	f := s.flow(p, flowName, fixedFlow)
	s.saveDraft(f, 0, document)
	s.publish(f, 1)
	s.activate(f)
	return f
}

// executionSucceeded seeds a finished execution of the greeting flow.
func executionSucceeded(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	s := newSeeder(ctx, api)
	f := activeFlowOf(s, greetingDocument(flowName))
	e := s.run(f, 10, fixedExec)
	s.firstNode(e, fixedNodeExec)
	return s.result()
}

// executionWaiting seeds an execution parked on a signal wait.
func executionWaiting(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	s := newSeeder(ctx, api)
	f := activeFlowOf(s, signalWaitDocument(flowName))
	s.run(f, 0, fixedExec)
	return s.result()
}

// executionFailed seeds an execution that failed at a call the egress policy refuses.
func executionFailed(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	s := newSeeder(ctx, api)
	f := activeFlowOf(s, failingDocument(flowName))
	s.run(f, 10, fixedExec)
	return s.result()
}

// executionWithHumanTask cannot be seeded yet: no operation of the public API
// lists the human tasks of an execution, so the id of the task cannot be
// discovered. The listing arrives with the human-attention operations
// (SPEC-005 API2-attn); the flow it needs is humanTaskDocument.
func executionWithHumanTask(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	_ = ctx
	_ = api
	_ = humanTaskDocument
	return nil, fmt.Errorf("execution with a human task: %w: no operation lists human tasks until API2-attn", ErrNotSeedable)
}

// executionWithUncertainOperation cannot be seeded yet: an uncertain external
// operation needs a connector that loses its answer, which the simulated
// provider of SPEC-003 supplies, and an operation that lists external
// operations.
func executionWithUncertainOperation(ctx context.Context, api *examplestest.Client) (examplestest.Bindings, error) {
	_ = ctx
	_ = api
	return nil, fmt.Errorf("execution with an uncertain operation: %w: needs the simulated provider of SPEC-003 and an external operation listing", ErrNotSeedable)
}
