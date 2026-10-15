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

export const productApi = {
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
