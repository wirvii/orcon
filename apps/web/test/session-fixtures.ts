// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, type HttpResponseResolver } from 'msw';

/**
 * Fixtures and handlers of the session API (SPEC-016 §3.1) for the screen tests. They are shaped
 * like the contract: a Session never carries the token, and a problem is `application/problem+json`.
 */

export const sessionId = '0195f1a0-0000-7000-8000-0000000000a1';

export const currentUser = {
  id: '0195f1a0-0000-7000-8000-0000000000b1',
  email: 'ana@example.com',
  displayName: 'Ana Ruiz',
  currentSession: {
    id: sessionId,
    createdAt: '2026-10-14T10:00:00Z',
    expiresAt: '2026-10-15T10:00:00Z',
    idleExpiresAt: '2026-10-14T12:00:00Z',
    assuranceLevel: 'AAL1',
  },
} as const;

export const acme = { id: '0195f1a0-0000-7000-8000-0000000000c1', slug: 'acme', name: 'Acme' } as const;
export const globex = { id: '0195f1a0-0000-7000-8000-0000000000c2', slug: 'globex', name: 'Globex' } as const;

/** The session created by a successful sign-in. */
export const createdSession = currentUser.currentSession;

/** A problem+json answer with the given status, code and title. */
export function problem(status: number, title: string, code?: string, extra: Record<string, string> = {}) {
  const body = { type: 'about:blank', status, title, ...(code === undefined ? {} : { code }) };
  return HttpResponse.json(body, { status, headers: { 'Content-Type': 'application/problem+json', ...extra } });
}

export const signedIn = {
  currentUser: http.get('/api/v1/users/me', () => HttpResponse.json(currentUser)),
  /** Answers `listOrgs`, filtering by `slug` the way the server does. */
  organizations: (items: readonly { id: string; slug: string; name: string }[]) =>
    http.get('/api/v1/orgs', ({ request }) => {
      const slug = new URL(request.url).searchParams.get('slug');
      return HttpResponse.json({ items: slug === null ? items : items.filter((item) => item.slug === slug) });
    }),
  createSession: (resolver: HttpResponseResolver = () => HttpResponse.json(createdSession, { status: 201 })) =>
    http.post('/api/v1/users/me/sessions', resolver),
  revokeSession: () => http.post(`/api/v1/users/me/sessions/${sessionId}/revoke`, () => new HttpResponse(null, { status: 204 })),
};
