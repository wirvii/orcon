// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiRequest } from '../../../app/api/http';

/** The response shapes of SPEC-016 §3.1 (the Session never carries the token). */
export const sessionSchema = z.object({
  id: z.uuid(),
  createdAt: z.string(),
  expiresAt: z.string(),
  idleExpiresAt: z.string(),
  assuranceLevel: z.enum(['AAL1', 'AAL2']),
});

export const currentUserSchema = z.object({
  id: z.uuid(),
  email: z.string(),
  displayName: z.string(),
  currentSession: sessionSchema,
});

export const organizationSchema = z.object({
  id: z.uuid(),
  slug: z.string(),
  name: z.string(),
});

export const organizationPageSchema = z.object({
  items: z.array(organizationSchema),
  nextPageToken: z.string().max(1024).optional(),
});

export type Session = z.infer<typeof sessionSchema>;
export type CurrentUser = z.infer<typeof currentUserSchema>;
export type Organization = z.infer<typeof organizationSchema>;
export type OrganizationPage = z.infer<typeof organizationPageSchema>;

export interface SessionCredentials {
  email: string;
  password: string;
}

/** Signs in with email and password (`createSession`). The server sets the session and CSRF cookies. */
export function createSession(credentials: SessionCredentials): Promise<Session> {
  return apiRequest({
    method: 'POST',
    path: '/users/me/sessions',
    body: { email: credentials.email, password: credentials.password },
    schema: sessionSchema,
    source: 'response of POST /users/me/sessions',
  });
}

/** Signs out the session of the caller (`revokeSession`). Only the current session can be revoked. */
export function revokeSession(sessionId: string): Promise<void> {
  return apiRequest({
    method: 'POST',
    path: `/users/me/sessions/${encodeURIComponent(sessionId)}/revoke`,
    source: 'response of POST /users/me/sessions/{sessionId}/revoke',
  });
}

/** The signed-in person (`getCurrentUser`). Answers 401 without a valid session. */
export function getCurrentUser(): Promise<CurrentUser> {
  return apiRequest({
    method: 'GET',
    path: '/users/me',
    schema: currentUserSchema,
    source: 'response of GET /users/me',
  });
}

/**
 * The organizations the caller is an active member of (`listOrgs`). The server reads at most 1000
 * of them; the page size is 200 at most (SPEC-016 R-SH-ORG-03).
 */
export function listOrganizations({ slug, limit = 200 }: { slug?: string | undefined; limit?: number | undefined }): Promise<OrganizationPage> {
  return apiRequest({
    method: 'GET',
    path: '/orgs',
    query: { slug, limit },
    schema: organizationPageSchema,
    source: 'response of GET /orgs',
  });
}
