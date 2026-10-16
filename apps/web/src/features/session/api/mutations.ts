// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { mutationOptions, useMutation, useQueryClient } from '@tanstack/react-query';

import type { Session } from '../../../app/session/current-user';
import { createSession, type SessionCredentials } from './operations';
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
