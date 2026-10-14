// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Page } from '@playwright/test';

import { EXAMPLE_FLOW_ID, exampleFlowDocument, SAMPLE_SUBFLOWS, subflowDocument } from '../src/features/editor/model/example-flow';

interface StoredDraft {
  revision: number;
  document: unknown;
}

/**
 * Answers the draft API of the sample flows in memory, the way the server does (SPEC-005 `getDraft`
 * and `replaceDraft`): the ETag is the revision, a save whose `If-Match` is not the current revision is
 * refused with 412, and an unknown flow is 404. The production build has no server in these tests, so
 * the editor reads and saves its drafts against this page-level answer.
 */
export async function mockDraftApi(page: Page): Promise<void> {
  const drafts = new Map<string, StoredDraft>([
    [EXAMPLE_FLOW_ID, { revision: 1, document: exampleFlowDocument() }],
    ...SAMPLE_SUBFLOWS.map((flow): [string, StoredDraft] => [flow.id, { revision: 1, document: subflowDocument(flow.id, flow.name) }]),
  ]);

  await page.route('**/api/v1/flows/*/draft', async (route) => {
    const flowId = new URL(route.request().url()).pathname.split('/')[4] ?? '';
    const draft = drafts.get(flowId);
    if (draft === undefined) {
      await route.fulfill({
        status: 404,
        contentType: 'application/problem+json',
        body: JSON.stringify({ type: 'about:blank', status: 404, title: 'Flow not found', code: 'not-found' }),
      });
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
      await route.fulfill({
        status: 412,
        contentType: 'application/problem+json',
        body: JSON.stringify({ type: 'about:blank', status: 412, title: 'The draft changed', code: 'precondition-failed' }),
      });
      return;
    }
    const body = JSON.parse(route.request().postData() ?? '{}') as { document: unknown };
    draft.document = body.document;
    draft.revision += 1;
    await route.fulfill(answer());
  });
}

/**
 * Moves to an address inside the running app without reloading the page, the way a link does, so the
 * query cache and the editor state survive the move. The sample navigation that linked the editor is gone.
 */
export async function goInApp(page: Page, path: string): Promise<void> {
  await page.evaluate((to) => {
    const state: unknown = window.history.state;
    window.history.pushState(state, '', to);
    window.dispatchEvent(new PopStateEvent('popstate', { state }));
  }, path);
}
