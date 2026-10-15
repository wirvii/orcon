// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query';

import type { NewCredential } from '../../../app/credential-store/store';

import { credentialStore } from './store';

/**
 * Every credential list is stale after a change. The flow editor keeps its own copy of the list for its
 * widget, under a key that also names credentials, so one invalidation reaches both.
 */
function invalidateCredentials(queryClient: QueryClient) {
  return queryClient.invalidateQueries({ predicate: (query) => query.queryKey.includes('credentials') });
}

/** Creates a credential in the project. Never retried: a retry could create a second one. */
export function useCreateCredential(projectId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: NewCredential) => credentialStore.create(projectId, input),
    retry: false,
    onSuccess: () => invalidateCredentials(queryClient),
  });
}

/** Replaces the secret values of a credential: a new version. Never retried. */
export function useRotateCredential() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ credentialId, fields }: { credentialId: string; fields: Readonly<Record<string, string>> }) => credentialStore.rotate(credentialId, fields),
    retry: false,
    onSuccess: () => invalidateCredentials(queryClient),
  });
}

/** Deletes a credential. Flows that use it fail at their next step, so the screen asks first. */
export function useDeleteCredential() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (credentialId: string) => credentialStore.remove(credentialId),
    retry: false,
    onSuccess: () => invalidateCredentials(queryClient),
  });
}
