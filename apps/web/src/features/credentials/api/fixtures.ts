// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, type HttpResponseResolver } from 'msw';

import { project } from '../../../../test/product-fixtures';

/** Fixtures and handlers of the credential API (`listCredentials`, `getCredential`, ...) for the screen tests. */

export const siigo = {
  id: '0195f1a0-0000-7000-8000-0000000000c1',
  projectId: project.id,
  owned: true,
  name: 'Siigo API',
  type: 'orcon.http@1',
  status: 'OK',
  allowedHosts: ['api.siigo.com', '*.siigo.com'],
  versionCount: 2,
  secret: {
    set: true,
    updatedAt: '2026-10-12T15:04:05Z',
    updatedBy: '0195f1a0-0000-7000-8000-0000000000a1',
  },
  createdAt: '2026-10-10T09:00:00Z',
  updatedAt: '2026-10-12T15:04:05Z',
} as const;

export const shared = {
  ...siigo,
  id: '0195f1a0-0000-7000-8000-0000000000c2',
  projectId: '0195f1a0-0000-7000-8000-0000000000d9',
  owned: false,
  name: 'Shared Slack token',
  allowedHosts: ['hooks.slack.com'],
} as const;

/** The detail of a credential carries its public fields. */
export const siigoDetail = {
  ...siigo,
  publicFields: { headerName: 'X-API-Key' },
} as const;

const listPath = `/api/v1/projects/${project.id}/credentials`;

export const credentialApi = {
  list: (items: readonly object[]) => http.get(listPath, () => HttpResponse.json({ items })),
  listWith: (resolver: HttpResponseResolver) => http.get(listPath, resolver),
  /** `getCredential`, with the entity tag the writes must send back. */
  get: (credential: { id: string }, etag = '"2-1"', body: object = credential) => http.get(`/api/v1/credentials/${credential.id}`, () => HttpResponse.json(body, { headers: { ETag: etag } })),
  nodeTypes: (items: readonly object[]) => http.get(`/api/v1/projects/${project.id}/node-types`, () => HttpResponse.json({ items })),
  nodeTypesFail: () => http.get(`/api/v1/projects/${project.id}/node-types`, () => new HttpResponse(null, { status: 503 })),
};

/** A node type with a brand icon, as `listNodeTypes` answers it for the HTTP step. */
export const httpNodeType = {
  type: 'orcon.http@1',
  name: 'HTTP request',
  group: 'Core',
  seal: 'OFFICIAL',
  proofLevel: 'TESTED',
  operations: [],
  icon: {
    svg: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 2a10 10 0 1 0 0 20z"/></svg>',
    source: 'orcon-fallback',
  },
};
