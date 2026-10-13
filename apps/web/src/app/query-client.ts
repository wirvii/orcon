// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { QueryClient } from '@tanstack/react-query';

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

/** The one query client of the app, with the defaults of orcon-web DT-7. */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: STALE_TIME_MS,
        gcTime: GC_TIME_MS,
        retry: shouldRetry,
        refetchOnWindowFocus: true,
      },
    },
  });
}

/**
 * Every query key starts with the organization id, so data of one organization is never
 * shown for another: switching organization drops the whole cache (orcon-web DT-4).
 */
export function clearCacheForOrganizationSwitch(queryClient: QueryClient): void {
  queryClient.clear();
}
