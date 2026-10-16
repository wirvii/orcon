// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, type HttpResponseResolver } from 'msw';

import { flow, project } from './product-fixtures';

/** Fixtures and handlers of the execution operations (SPEC-005 `listExecutions`, `getExecution`, `listExecutionNodes`, `listExecutionData`). */

export const runId = '0195f1a0-0000-7000-8000-0000000000f1';
export const otherRunId = '0195f1a0-0000-7000-8000-0000000000f2';

export const run = {
  id: runId,
  flowId: flow.id,
  projectId: project.id,
  flowVersion: 2,
  mode: 'MANUAL',
  status: 'FAILED',
  createdAt: '2026-10-14T09:00:00Z',
  startedAt: '2026-10-14T09:00:01Z',
  finishedAt: '2026-10-14T09:00:04Z',
  failure: { code: 'node-failed', title: 'The step failed', detail: 'The webhook answered 500.', nodeId: 'slack' },
  dataHidden: false,
} as const;

export const otherRun = {
  id: otherRunId,
  flowId: flow.id,
  projectId: project.id,
  flowVersion: 2,
  mode: 'WEBHOOK',
  status: 'SUCCEEDED',
  createdAt: '2026-10-14T08:00:00Z',
  startedAt: '2026-10-14T08:00:00Z',
  finishedAt: '2026-10-14T08:00:02Z',
  dataHidden: false,
} as const;

export const greetingStep = {
  id: '0195f1a0-0000-7000-8000-0000000000a1',
  executionId: runId,
  nodeId: 'greeting',
  status: 'SUCCEEDED',
  attemptCount: 1,
  startedAt: '2026-10-14T09:00:01Z',
  finishedAt: '2026-10-14T09:00:01Z',
} as const;

export const slackStep = {
  id: '0195f1a0-0000-7000-8000-0000000000a2',
  executionId: runId,
  nodeId: 'slack',
  status: 'FAILED',
  attemptCount: 3,
  startedAt: '2026-10-14T09:00:02Z',
  finishedAt: '2026-10-14T09:00:04Z',
  failure: { code: 'node-failed', detail: 'The webhook answered 500.' },
} as const;

export const greetingData = {
  nodeExecutionId: greetingStep.id,
  nodeId: 'greeting',
  side: 'OUTPUT',
  port: 'main',
  itemCount: 1,
  dataItems: [{ text: 'Hello, Ana' }],
} as const;

export const executionApi = {
  /** `listExecutions` of the project: pages by `pageToken`, filtered by `flowId` as the server does. */
  list: (pages: readonly (readonly object[])[]) =>
    http.get(`/api/v1/projects/${project.id}/executions`, ({ request }) => {
      const url = new URL(request.url);
      const token = url.searchParams.get('pageToken');
      const index = token === null ? 0 : Number(token);
      const items = pages[index] ?? [];
      const next = index + 1 < pages.length ? String(index + 1) : undefined;
      return HttpResponse.json({ items, ...(next === undefined ? {} : { nextPageToken: next }) });
    }),
  run: (answer: HttpResponseResolver) => http.get(`/api/v1/executions/${runId}`, answer),
  nodes: (items: readonly object[]) => http.get(`/api/v1/executions/${runId}/nodes`, () => HttpResponse.json({ items })),
  data: (items: readonly object[]) => http.get(`/api/v1/executions/${runId}/data`, () => HttpResponse.json({ items })),
};
