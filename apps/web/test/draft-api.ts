// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Page } from '@playwright/test';

import { EXAMPLE_FLOW_ID, exampleFlowDocument, SAMPLE_SUBFLOWS, subflowDocument } from '../src/features/editor/model/example-flow';

interface StoredDraft {
  revision: number;
  document: unknown;
}

interface StoredFlow {
  activationState: 'ACTIVE' | 'INACTIVE';
  activeVersion: number | null;
  hasUnpublishedChanges: boolean;
  /** The entity tag of the flow resource: it changes with every change to the flow. */
  tag: number;
  versions: number;
}

/** The problem a refused change answers with, as the server shapes it. */
const problemBody = (status: number, title: string, code: string) => ({
  status,
  contentType: 'application/problem+json',
  body: JSON.stringify({ type: 'about:blank', status, title, code }),
});

/** The id of the flow an address names: `/api/v1/flows/<id>[/...]`. */
const flowIdOf = (url: string) => new URL(url).pathname.split('/')[4] ?? '';

/**
 * Answers the flow API of the sample flows in memory, the way the server does (SPEC-005): the draft
 * (`getDraft`, `replaceDraft`) with its revision as the ETag, a save whose `If-Match` is not the current
 * revision is refused with 412, the flow state (`getFlow`), publishing, and activation or deactivation
 * against the entity tag of the flow. An unknown flow is 404. The production build has no server in these
 * tests, so the editor reads and changes its flows against this page-level answer.
 */
export async function mockDraftApi(page: Page): Promise<void> {
  const drafts = new Map<string, StoredDraft>([
    [EXAMPLE_FLOW_ID, { revision: 1, document: exampleFlowDocument() }],
    ...SAMPLE_SUBFLOWS.map((flow): [string, StoredDraft] => [flow.id, { revision: 1, document: subflowDocument(flow.id, flow.name) }]),
  ]);
  const flows = new Map<string, StoredFlow>(
    [EXAMPLE_FLOW_ID, ...SAMPLE_SUBFLOWS.map((flow) => flow.id)].map((id): [string, StoredFlow] => [
      id,
      { activationState: 'INACTIVE', activeVersion: null, hasUnpublishedChanges: true, tag: 1, versions: 0 },
    ]),
  );

  await page.route('**/api/v1/flows/*/draft', async (route) => {
    const flowId = flowIdOf(route.request().url());
    const draft = drafts.get(flowId);
    if (draft === undefined) {
      await route.fulfill(problemBody(404, 'Flow not found', 'not-found'));
      return;
    }
    const answer = () => ({
      status: 200,
      contentType: 'application/json',
      headers: { ETag: `"${draft.revision}"` },
      body: JSON.stringify({ flowId, revision: draft.revision, document: draft.document, updatedAt: '2026-10-14T10:00:00Z' }),
    });
    if (route.request().method() === 'GET') {
      await route.fulfill(answer());
      return;
    }
    if (route.request().headers()['if-match'] !== `"${draft.revision}"`) {
      await route.fulfill(problemBody(412, 'The draft changed', 'precondition-failed'));
      return;
    }
    const body = JSON.parse(route.request().postData() ?? '{}') as { document: unknown };
    draft.document = body.document;
    draft.revision += 1;
    const flow = flows.get(flowId);
    if (flow !== undefined) flow.hasUnpublishedChanges = true;
    await route.fulfill(answer());
  });

  const stateBody = (flowId: string, flow: StoredFlow) => ({
    id: flowId,
    projectId: '0195f1a0-0000-7000-8000-0000000000d1',
    name: 'Order notifications',
    activationState: flow.activationState,
    activeVersion: flow.activeVersion,
    hasUnpublishedChanges: flow.hasUnpublishedChanges,
    draftRevision: drafts.get(flowId)?.revision ?? 1,
    createdAt: '2026-10-10T09:00:00Z',
    updatedAt: '2026-10-14T10:00:00Z',
  });

  await page.route('**/api/v1/flows/*', async (route) => {
    const flowId = flowIdOf(route.request().url());
    const flow = flows.get(flowId);
    if (flow === undefined) {
      await route.fulfill(problemBody(404, 'Flow not found', 'not-found'));
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { ETag: `"${flow.tag}"` },
      body: JSON.stringify(stateBody(flowId, flow)),
    });
  });

  await page.route('**/api/v1/flows/*/publish', async (route) => {
    const flowId = flowIdOf(route.request().url());
    const flow = flows.get(flowId);
    if (flow === undefined) {
      await route.fulfill(problemBody(404, 'Flow not found', 'not-found'));
      return;
    }
    flow.versions += 1;
    flow.activeVersion = flow.versions;
    flow.hasUnpublishedChanges = false;
    flow.tag += 1;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ flowId, version: flow.versions, contentHash: `sha256:${flowId.slice(-8)}`, publishedAt: '2026-10-14T10:00:00Z' }),
    });
  });

  await page.route(/\/api\/v1\/flows\/[^/]+\/(activate|deactivate)$/, async (route) => {
    const flowId = flowIdOf(route.request().url());
    const flow = flows.get(flowId);
    if (flow === undefined) {
      await route.fulfill(problemBody(404, 'Flow not found', 'not-found'));
      return;
    }
    if (route.request().headers()['if-match'] !== `"${flow.tag}"`) {
      await route.fulfill(problemBody(412, 'The flow changed', 'precondition-failed'));
      return;
    }
    flow.activationState = route.request().url().endsWith('/activate') ? 'ACTIVE' : 'INACTIVE';
    flow.tag += 1;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { ETag: `"${flow.tag}"` },
      body: JSON.stringify(stateBody(flowId, flow)),
    });
  });
}

/**
 * Moves to an address inside the running app without reloading the page, the way a link does, so the
 * query cache and the editor state survive the move.
 */
export async function goInApp(page: Page, path: string): Promise<void> {
  await page.evaluate((to) => {
    const state: unknown = window.history.state;
    window.history.pushState(state, '', to);
    window.dispatchEvent(new PopStateEvent('popstate', { state }));
  }, path);
}

/** True for a request the editor makes for its own flow: its state or its draft. Anything else is not expected. */
export const isFlowDataRequest = (url: string): boolean => /\/api\/v1\/flows\/[^/]+(\/draft)?$/.test(url);
