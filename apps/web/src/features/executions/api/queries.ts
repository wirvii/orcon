// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { infiniteQueryOptions, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';

import { rangeStart, type ExecutionsSearch } from '../model/params';

import { getExecution, listExecutionData, listExecutionNodes, listOrgExecutions, listVersionNodes, retryExecution, type ExecutionFilters } from './operations';

/**
 * Keys of the run data (orcon-web DT-3, DT-4): every key starts with the active organization id, so the runs of one
 * organization are never shown for another.
 */
export const executionKeys = {
  all: (orgId: string) => [orgId, 'executions'] as const,
  list: (orgId: string, search: ExecutionsSearch) =>
    [orgId, 'executions', 'list', search.projectId ?? null, search.flowId ?? null, search.show ?? null, search.range ?? null, search.q ?? null] as const,
  detail: (orgId: string, executionId: string) => [orgId, 'executions', 'detail', executionId] as const,
  nodes: (orgId: string, executionId: string) => [orgId, 'executions', 'nodes', executionId] as const,
  data: (orgId: string, executionId: string) => [orgId, 'executions', 'data', executionId] as const,
  version: (orgId: string, flowId: string, version: number) => [orgId, 'executions', 'version', flowId, version] as const,
};

/** What the address asks for, as the filters of the operation. The start of a date range is counted when the read happens. */
function filtersOf(search: ExecutionsSearch, now: Date): ExecutionFilters {
  return {
    projectId: search.projectId,
    flowId: search.flowId,
    status: search.show === 'failed' ? 'FAILED' : undefined,
    uncertain: search.show === 'review' ? true : undefined,
    createdAfter: search.range === undefined ? undefined : rangeStart(search.range, now),
    id: search.q,
  };
}

/** The runs the address filters, one page at a time (DT-10): the next page is asked for when the person asks for it. */
export function executionsInfiniteOptions(orgId: string, search: ExecutionsSearch) {
  return infiniteQueryOptions({
    queryKey: executionKeys.list(orgId, search),
    queryFn: ({ pageParam }) => listOrgExecutions({ orgId, filters: filtersOf(search, new Date()), pageToken: pageParam }),
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

/**
 * The nodes of the published version a run executed: they give the steps their names and say which never ran. A
 * version never changes, so it is read once. It is not retried: when it cannot be read the page names steps by id.
 */
export function versionNodesQueryOptions(orgId: string, flowId: string, version: number) {
  return queryOptions({
    queryKey: executionKeys.version(orgId, flowId, version),
    queryFn: () => listVersionNodes(flowId, version),
    staleTime: Infinity,
    retry: false,
  });
}

/** Starts a retry of a failed run. It is never retried by itself: a second request would start a second run. */
export function useRetryExecution(orgId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationKey: [...executionKeys.all(orgId), 'retry'],
    mutationFn: (executionId: string) => retryExecution(executionId),
    retry: false,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: executionKeys.all(orgId) }),
  });
}
