// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query';

import { createCredential, deleteCredential, getCredential, replaceSecret, updateAllowedHosts } from './operations';
import type { NewCredential } from './schema';

/**
 * Every credential list is stale after a change. The flow editor keeps its own copy of the list for its
 * widget, under a key that also names credentials, so one invalidation reaches both.
 */
function invalidateCredentials(queryClient: QueryClient) {
  return queryClient.invalidateQueries({
    predicate: (query) => query.queryKey.includes('credentials'),
  });
}

/**
 * Creates a credential in the project. Never retried: a retry could create a second one. The secret values are in
 * the input only for the call, so the mutation leaves the cache as soon as it ends (`gcTime: 0`, SPEC-010 FM-8).
 */
export function useCreateCredential(projectId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: NewCredential) => createCredential(projectId, input),
    retry: false,
    gcTime: 0,
    onSuccess: () => invalidateCredentials(queryClient),
  });
}

/** Replaces the secret of a credential: a new version. It sends the tag the person's dialog was opened on, so a change by someone else is refused. */
export function useReplaceSecret() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      credentialId,
      etag,
      secretFields,
      publicFields,
    }: {
      credentialId: string;
      etag: string;
      secretFields: Readonly<Record<string, string>>;
      publicFields?: Readonly<Record<string, string>> | undefined;
    }) => replaceSecret(credentialId, etag, { secretFields, publicFields }),
    retry: false,
    gcTime: 0,
    onSuccess: () => invalidateCredentials(queryClient),
  });
}

/** Changes the hosts the secret may be sent to. Same tag rule as the secret. */
export function useUpdateAllowedHosts() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ credentialId, etag, allowedHosts }: { credentialId: string; etag: string; allowedHosts: readonly string[] }) => updateAllowedHosts(credentialId, etag, allowedHosts),
    retry: false,
    onSuccess: () => invalidateCredentials(queryClient),
  });
}

/**
 * Deletes a credential. Flows that use it fail at their next step, so the screen asks first; the person
 * confirmed this credential, so the tag is read at that moment.
 */
export function useDeleteCredential() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (credentialId: string) => {
      const { etag } = await getCredential(credentialId);
      await deleteCredential(credentialId, etag);
    },
    retry: false,
    onSuccess: () => invalidateCredentials(queryClient),
  });
}
