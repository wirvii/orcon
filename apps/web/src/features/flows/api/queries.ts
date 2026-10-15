// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';

import { listFlows } from './operations';

/** Keys of the flow data (orcon-web DT-3, DT-4): they start with the active organization id. */
export const flowKeys = {
  all: (orgId: string) => [orgId, 'flows'] as const,
  list: (orgId: string, projectId: string) => [orgId, 'flows', 'list', projectId] as const,
};

/** The flows of a project, most recently changed first. */
export function flowsQueryOptions(orgId: string, projectId: string) {
  return queryOptions({
    queryKey: flowKeys.list(orgId, projectId),
    queryFn: async () => (await listFlows({ projectId })).items,
  });
}
