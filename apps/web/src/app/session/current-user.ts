// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { z } from 'zod';

import { apiRequest } from '../api/http';

/**
 * The signed-in person, read by the frame of every signed-in page and by several areas (home, settings, session).
 * It lives in `app/` because areas do not import each other (orcon-web AR-6). The keys start with the constant
 * `'session'`: signing out clears the whole cache.
 */

/** The response shape of SPEC-016 §3.1 (the Session never carries the token). */
export const sessionSchema = z.object({
  id: z.uuid(),
  createdAt: z.string(),
  expiresAt: z.string(),
  idleExpiresAt: z.string(),
  assuranceLevel: z.enum(['AAL1', 'AAL2']),
});

/** The predefined roles the server names in `OrgRole.role` (ADR-0041, SPEC-005), spelled as it sends them. */
export const organizationRoleSchema = z.enum(['Owner', 'Admin', 'Member', 'Project Admin', 'Editor', 'Operator', 'Viewer']);

export const currentUserSchema = z.object({
  id: z.uuid(),
  email: z.string(),
  displayName: z.string(),
  currentSession: sessionSchema,
  /** The role of the person in each of their organizations. The server sends it once the contract has it (SPEC-019 UXB1). */
  orgRoles: z.array(z.object({ orgId: z.uuid(), role: organizationRoleSchema })).optional(),
});

export type Session = z.infer<typeof sessionSchema>;
export type OrganizationRole = z.infer<typeof organizationRoleSchema>;
export type CurrentUser = z.infer<typeof currentUserSchema>;

/** Keys of the data read before an organization is active (orcon-web DT-4). */
export const sessionKeyRoot = ['session'] as const;
const currentUserKey = [...sessionKeyRoot, 'current-user'] as const;

/** The signed-in person (`getCurrentUser`). Answers 401 without a valid session. */
export function getCurrentUser(): Promise<CurrentUser> {
  return apiRequest({
    method: 'GET',
    path: '/users/me',
    schema: currentUserSchema,
    source: 'response of GET /users/me',
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

/**
 * The signed-in person. It keeps the default freshness, so a session that ended on the server is noticed
 * by the next request, and a 401 from any request sends the person to sign-in (see `onUnauthenticated`).
 */
export function currentUserQueryOptions() {
  return queryOptions({
    queryKey: currentUserKey,
    queryFn: getCurrentUser,
  });
}

/**
 * Signs out: revokes the session, goes to the sign-in page and only then clears the cache. The
 * cache is cleared whatever the answer: a session the server no longer knows (401) is signed out
 * here too, and nothing read for that person may stay in memory. The page is left first, so no
 * query of the signed-in pages refetches with a session that is gone.
 */
export function useSignOut() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const mutation = useMutation({
    mutationKey: [...sessionKeyRoot, 'revoke'],
    mutationFn: (sessionId: string) => revokeSession(sessionId),
    retry: false,
  });
  return async (sessionId: string) => {
    try {
      await mutation.mutateAsync(sessionId);
    } catch {
      // The outcome is the same for the person: the page they are on can no longer show their data.
    }
    await navigate({ to: '/login', search: { signedOut: true } });
    queryClient.clear();
  };
}
