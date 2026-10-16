// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, type HttpResponseResolver } from 'msw';

import { flow, project } from './product-fixtures';
import { acme } from './session-fixtures';

/** Fixtures and handlers of the execution operations (`listOrgExecutions`, `getExecution`, `listExecutionNodes`, `listExecutionData`, `retryExecution`). */

export const runId = '0195f1a0-0000-7000-8000-0000000000f1';
export const otherRunId = '0195f1a0-0000-7000-8000-0000000000f2';

export const run = {
  id: runId,
  flowId: flow.id,
  projectId: project.id,
  flowVersion: 2,
  mode: 'MANUAL',
  status: 'FAILED',
  hasUncertainOperation: false,
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
  hasUncertainOperation: false,
  createdAt: '2026-10-14T08:00:00Z',
  startedAt: '2026-10-14T08:00:00Z',
  finishedAt: '2026-10-14T08:00:02Z',
  dataHidden: false,
} as const;

export const reviewRunId = '0195f1a0-0000-7000-8000-0000000000f3';

/** A run that finished but has an operation whose outcome is unknown: it needs a person to review it. */
export const reviewRun = {
  id: reviewRunId,
  flowId: flow.id,
  projectId: project.id,
  flowVersion: 2,
  mode: 'TRIGGER',
  status: 'SUCCEEDED',
  hasUncertainOperation: true,
  createdAt: '2026-10-14T10:00:00Z',
  startedAt: '2026-10-14T10:00:01Z',
  finishedAt: '2026-10-14T10:00:03Z',
  dataHidden: false,
} as const;

/** The nodes of version 2 of the flow: three steps, the last of which the failed run never reached. */
export const versionTwo = {
  flowId: flow.id,
  version: 2,
  contentHash: 'sha256:aaaa',
  publishedAt: '2026-10-13T16:00:00Z',
  warnings: [],
  document: {
    format: 'orcon/flow@1',
    nodes: [
      { id: 'greeting', name: 'Greet the customer', type: 'set' },
      { id: 'slack', name: 'Notify the team', type: 'http' },
      { id: 'mark', name: 'Mark as notified', type: 'set' },
    ],
    edges: [],
  },
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
  /**
   * `listOrgExecutions`: pages by `pageToken`. It records the query of every request in `seen`, so a test can say which filters
   * the screen sent, and it answers the `uncertain` and `id` filters the way the server does.
   */
  list: (pages: readonly (readonly object[])[], seen: URLSearchParams[] = []) =>
    http.get(`/api/v1/orgs/${acme.id}/executions`, ({ request }) => {
      const url = new URL(request.url);
      seen.push(url.searchParams);
      const token = url.searchParams.get('pageToken');
      const index = token === null ? 0 : Number(token);
      const keep = (item: object) => {
        const row = item as { id: string; hasUncertainOperation?: boolean };
        if (url.searchParams.get('uncertain') === 'true' && row.hasUncertainOperation !== true) return false;
        const id = url.searchParams.get('id');
        return id === null || row.id === id;
      };
      const items = (pages[index] ?? []).filter(keep);
      const next = index + 1 < pages.length ? String(index + 1) : undefined;
      return HttpResponse.json({ items, ...(next === undefined ? {} : { nextPageToken: next }) });
    }),
  /** `getExecution` of the first run of the fixtures. */
  run: (answer: HttpResponseResolver, id: string = runId) => http.get(`/api/v1/executions/${id}`, answer),
  nodes: (items: readonly object[], id: string = runId) => http.get(`/api/v1/executions/${id}/nodes`, () => HttpResponse.json({ items })),
  data: (items: readonly object[], id: string = runId) => http.get(`/api/v1/executions/${id}/data`, () => HttpResponse.json({ items })),
  /** `getFlowVersion`: the nodes of the version the runs executed. */
  version: (answer: HttpResponseResolver = () => HttpResponse.json(versionTwo)) => http.get(`/api/v1/flows/${flow.id}/versions/2`, answer),
  /** `retryExecution` of a run. */
  retry: (answer: HttpResponseResolver, id: string = runId) => http.post(`/api/v1/executions/${id}/retry`, answer),
};
