// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';

import { getOrgActivitySummary } from './operations';

/** Keys of the home page figures (orcon-web DT-3, DT-4): they start with the id of the active organization. */
export const overviewKeys = {
  summary: (orgId: string) => [orgId, 'activity', 'summary'] as const,
};

/**
 * The activity summary of the organization. It is not retried: when the server says the figures are not
 * available (503) the page hides them, and waiting would only delay the list under them.
 */
export function activitySummaryQueryOptions(orgId: string) {
  return queryOptions({
    queryKey: overviewKeys.summary(orgId),
    queryFn: () => getOrgActivitySummary(orgId),
    retry: false,
    staleTime: 30_000,
  });
}
