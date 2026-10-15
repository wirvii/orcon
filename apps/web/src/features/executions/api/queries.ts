// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query';

import { getExecution, listExecutionData, listExecutionNodes, listExecutions } from './operations';

/**
 * Keys of the run data (orcon-web DT-3, DT-4): every key starts with the active organization id, so the
 * runs of one organization are never shown for another.
 */
export const executionKeys = {
  all: (orgId: string) => [orgId, 'executions'] as const,
  list: (orgId: string, projectId: string, flowId: string | undefined) => [orgId, 'executions', 'list', projectId, flowId ?? null] as const,
  detail: (orgId: string, executionId: string) => [orgId, 'executions', 'detail', executionId] as const,
  nodes: (orgId: string, executionId: string) => [orgId, 'executions', 'nodes', executionId] as const,
  data: (orgId: string, executionId: string) => [orgId, 'executions', 'data', executionId] as const,
};

/** The runs of a project, one page at a time (DT-10): the next page is asked for when the person asks for it. */
export function executionsInfiniteOptions(orgId: string, projectId: string, flowId: string | undefined) {
  return infiniteQueryOptions({
    queryKey: executionKeys.list(orgId, projectId, flowId),
    queryFn: ({ pageParam }) => listExecutions({ projectId, flowId, pageToken: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.nextPageToken,
  });
}

/** One run. A run that is still going changes, so it is read again when the person comes back to it. */
export function executionQueryOptions(orgId: string, executionId: string) {
  return queryOptions({
    queryKey: executionKeys.detail(orgId, executionId),
    queryFn: () => getExecution(executionId),
    staleTime: 0,
  });
}

export function executionNodesQueryOptions(orgId: string, executionId: string) {
  return queryOptions({
    queryKey: executionKeys.nodes(orgId, executionId),
    queryFn: () => listExecutionNodes(executionId),
    staleTime: 0,
  });
}

/** The items of a run. Only asked for when the person may read them (`dataHidden` is false). */
export function executionDataQueryOptions(orgId: string, executionId: string) {
  return queryOptions({
    queryKey: executionKeys.data(orgId, executionId),
    queryFn: () => listExecutionData(executionId),
    staleTime: 0,
  });
}
