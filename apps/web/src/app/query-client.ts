// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query';

import { problemStatus } from './errors/to-problem-details';

/** Data stays fresh for 30 seconds and in the cache for 5 minutes (orcon-web DT-7). */
export const STALE_TIME_MS = 30_000;
export const GC_TIME_MS = 5 * 60_000;
/** A failed request is retried at most twice, and only when the network failed. */
export const MAX_RETRIES = 2;

/**
 * A request that never reached the server. `fetch` rejects with a TypeError then; any
 * answer from the server, including an error status, is an answer and is not retried here
 * (the SDK already retries 429 and 503).
 */
export function isNetworkFailure(error: unknown): boolean {
  return error instanceof TypeError;
}

export function shouldRetry(failureCount: number, error: unknown): boolean {
  return failureCount < MAX_RETRIES && isNetworkFailure(error);
}

const unauthenticatedHandlers = new WeakMap<QueryClient, () => void>();

/**
 * Registers what happens when the server answers 401 to a request of this client: the router sends the
 * person to sign-in with the address they were on (SPEC-016 R-SH-URL-02). Read before the first load, a
 * session can end at any time, so every query, loader and mutation reports here, not only the layout.
 */
export function onUnauthenticated(queryClient: QueryClient, handler: () => void): void {
  unauthenticatedHandlers.set(queryClient, handler);
}

function reportFailure(queryClient: QueryClient, error: unknown): void {
  if (problemStatus(error) === 401) unauthenticatedHandlers.get(queryClient)?.();
}

/** A session mutation answers 401 as a normal result (a wrong password, or a session already gone): it is not a lost session. */
function isSessionMutation(mutation: { options: { mutationKey?: readonly unknown[] | undefined } }): boolean {
  return mutation.options.mutationKey?.[0] === 'session';
}

/** The one query client of the app, with the defaults of orcon-web DT-7. */
export function createQueryClient(): QueryClient {
  const queryClient: QueryClient = new QueryClient({
    queryCache: new QueryCache({
      onError: (error) => {
        reportFailure(queryClient, error);
      },
    }),
    mutationCache: new MutationCache({
      onError: (error, _variables, _context, mutation) => {
        if (!isSessionMutation(mutation)) reportFailure(queryClient, error);
      },
    }),
    defaultOptions: {
      queries: {
        staleTime: STALE_TIME_MS,
        gcTime: GC_TIME_MS,
        retry: shouldRetry,
        refetchOnWindowFocus: true,
      },
    },
  });
  return queryClient;
}

/**
 * Every query key starts with the organization id, so data of one organization is never
 * shown for another: switching organization drops the whole cache (orcon-web DT-4).
 */
export function clearCacheForOrganizationSwitch(queryClient: QueryClient): void {
  queryClient.clear();
}
