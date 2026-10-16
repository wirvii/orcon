// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, type HttpResponseResolver } from 'msw';

import { acme } from './session-fixtures';

/** Fixtures and handlers of the projects and flows API (SPEC-016 §3.1, §3.4, §3.5) for the screen tests. */

export const project = {
  id: '0195f1a0-0000-7000-8000-0000000000d1',
  orgId: acme.id,
  name: 'Ventas',
  slug: 'ventas',
  handle: 'ventas',
  environment: 'production',
  protected: true,
  createdAt: '2026-10-10T09:00:00Z',
  updatedAt: '2026-10-10T09:00:00Z',
} as const;

export const flow = {
  id: '0195f1a0-0000-7000-8000-0000000000e1',
  projectId: project.id,
  name: 'Order notifications',
  activationState: 'INACTIVE',
  createdAt: '2026-10-10T09:30:00Z',
  updatedAt: '2026-10-13T16:00:00Z',
} as const;

/** A flow as `listOrgFlows` reads it: published, with its last execution and the runs of the last 24 hours (SPEC-019 UXB1). */
export const listedFlow = {
  ...flow,
  activationState: 'ACTIVE',
  activeVersion: 2,
  hasUnpublishedChanges: false,
  draftRevision: 3,
  lastExecution: { id: '0195f1a0-0000-7000-8000-0000000000f1', status: 'SUCCEEDED', hasUncertainOperation: false, createdAt: '2026-10-14T09:00:00Z', finishedAt: '2026-10-14T09:00:04Z' },
  executionCountLast24h: 12,
} as const;

/** The answer of `getOrgActivitySummary`: 18 flows, 12 active, 3 projects, 128 runs in 24 hours (SPEC-019 UXB2). */
export const activitySummary = {
  activeFlowCount: 12,
  flowCount: 18,
  projectCount: 3,
  executionsLast24h: { total: 128, failed: 3, succeeded: 120, uncertain: 2, byHour: [0, 0, 1, 4, 9, 12, 8, 6, 5, 7, 11, 13, 9, 8, 6, 5, 4, 3, 2, 1, 0, 0, 0, 5] },
  timeSaved: { windowDays: 7, succeededExecutions: 620, minutesPerExecution: 3, minutes: 1860 },
} as const;

export const productApi = {
  /** `getOrgActivitySummary` of the organization. */
  activitySummary: (resolver: HttpResponseResolver = () => HttpResponse.json(activitySummary)) => http.get(`/api/v1/orgs/${acme.id}/activity-summary`, resolver),
  /** `listOrgFlows` of the organization; answers the `projectId`, `activationState` and `q` filters the way the server does. */
  orgFlows: (items: readonly { projectId: string; activationState?: string; name: string }[] | readonly object[]) =>
    http.get(`/api/v1/orgs/${acme.id}/flows`, ({ request }) => {
      const query = new URL(request.url).searchParams;
      const rows = (items as readonly { projectId: string; activationState?: string; name: string }[])
        .filter((item) => query.get('projectId') === null || item.projectId === query.get('projectId'))
        .filter((item) => query.get('activationState') === null || item.activationState === query.get('activationState'))
        .filter((item) => query.get('q') === null || item.name.toLowerCase().startsWith((query.get('q') ?? '').toLowerCase()));
      return HttpResponse.json({ items: rows });
    }),
  /** `activateFlow` and `deactivateFlow` of one flow: they answer the flow in its new state. */
  setActivation: (flowId: string, resolver: HttpResponseResolver) => [
    http.post(`/api/v1/flows/${flowId}/activate`, resolver),
    http.post(`/api/v1/flows/${flowId}/deactivate`, resolver),
  ],
  /** `listProjects` of the organization; answers the `handle` filter too, as the server does. */
  projects: (items: readonly (typeof project)[] | readonly { handle: string; [key: string]: unknown }[]) =>
    http.get(`/api/v1/orgs/${acme.id}/projects`, ({ request }) => {
      const handle = new URL(request.url).searchParams.get('handle');
      return HttpResponse.json({ items: handle === null ? items : items.filter((item) => item.handle === handle) });
    }),
  /** `createProject` of the organization, with a resolver so a test can answer an error. */
  createProject: (resolver: HttpResponseResolver) => http.post(`/api/v1/orgs/${acme.id}/projects`, resolver),
  /** `listFlows` of a project. */
  flows: (items: readonly (typeof flow)[] | readonly object[]) => http.get(`/api/v1/projects/${project.id}/flows`, () => HttpResponse.json({ items })),
  /** `createFlow` of a project, with a resolver so a test can answer an error. */
  createFlow: (resolver: HttpResponseResolver) => http.post(`/api/v1/projects/${project.id}/flows`, resolver),
};
