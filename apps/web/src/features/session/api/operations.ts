// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiRequest } from '../../../app/api/http';
import { sessionSchema, type Session } from '../../../app/session/current-user';

export const organizationSchema = z.object({
  id: z.uuid(),
  slug: z.string(),
  name: z.string(),
});

export const organizationPageSchema = z.object({
  items: z.array(organizationSchema),
  nextPageToken: z.string().max(1024).optional(),
});

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
