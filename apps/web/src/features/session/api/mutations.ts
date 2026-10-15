// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { mutationOptions, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';

import { createSession, revokeSession, type Session, type SessionCredentials } from './operations';
import { sessionKeys } from './queries';

/**
 * Signs in. A sign-in is never retried: a wrong password must not be tried again by the client,
 * and a throttled answer is shown with the time the server asked to wait.
 */
export function signInMutationOptions() {
  return mutationOptions<Session, Error, SessionCredentials>({
    mutationKey: [...sessionKeys.all, 'create'],
    mutationFn: createSession,
    retry: false,
    // The password is in the mutation's variables only while the request runs: nothing keeps it afterwards.
    gcTime: 0,
  });
}

/**
 * Signs in and clears everything read before (orcon-web DT-4): what was read belongs to the session that
 * just ended. The caller decides where to go next (the landing reads the organizations of the new session).
 */
export function useSignIn() {
  const queryClient = useQueryClient();
  return useMutation({
    ...signInMutationOptions(),
    onSuccess: () => {
      queryClient.clear();
    },
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
    mutationKey: [...sessionKeys.all, 'revoke'],
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
