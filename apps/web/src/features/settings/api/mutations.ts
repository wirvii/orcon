// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useMutation, useQueryClient } from '@tanstack/react-query';

import { currentUserQueryOptions, revokeSession } from '../../../app/session/current-user';
import { updateDisplayName } from './operations';
import { accountKeys } from './queries';

/**
 * Changes the display name with the tag the form was opened on. The answer is the updated person, which
 * replaces the one the frame reads, so the sidebar and the greeting show the new name without another request.
 */
export function useRenameUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ displayName, etag }: { displayName: string; etag: string }) => updateDisplayName(displayName, etag),
    retry: false,
    onSuccess: ({ user }) => {
      queryClient.setQueryData(currentUserQueryOptions().queryKey, user);
      return queryClient.invalidateQueries({ queryKey: accountKeys.profile });
    },
  });
}

/**
 * Signs out one of the other sessions of the person. The list is read again whatever the answer: a session
 * that is already gone (404) must disappear from it too.
 */
export function useRevokeOtherSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) => revokeSession(sessionId),
    retry: false,
    onSettled: () => queryClient.invalidateQueries({ queryKey: accountKeys.sessions }),
  });
}
