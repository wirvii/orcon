// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';

import { apiCall, apiRequest } from '../api/http';

/**
 * The flows of an organization with their activity: the last execution and the runs of the last 24 hours
 * (`listOrgFlows`, SPEC-019 UXB1). The home page and the page of a project show the same rows, so the read, the
 * Active switch and the row live in `app/` (orcon-web AR-6): areas do not import each other. Every key starts with
 * the id of the active organization and sits under `[orgId, 'flows']`, so creating or changing a flow refreshes them.
 */

export const lastExecutionStatuses = ['QUEUED', 'RUNNING', 'WAITING', 'SUSPENDED', 'CANCELING', 'COMPENSATING', 'SUCCEEDED', 'FAILED', 'CANCELED'] as const;

export const listedFlowSchema = z.object({
  id: z.uuid(),
  projectId: z.uuid(),
  name: z.string(),
  activationState: z.enum(['ACTIVE', 'INACTIVE']),
  /** Absent until the first publication: a flow that was never published cannot be activated. */
  activeVersion: z.number().int().nullish(),
  hasUnpublishedChanges: z.boolean().optional(),
  /** The value of the entity tag of the flow (`If-Match` when it is activated or deactivated). */
  draftRevision: z.number().int().nonnegative().optional(),
  lastExecution: z
    .object({
      id: z.uuid(),
      status: z.enum(lastExecutionStatuses),
      hasUncertainOperation: z.boolean(),
      createdAt: z.string(),
      finishedAt: z.string().optional(),
    })
    .optional(),
  executionCountLast24h: z.number().int().nonnegative().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const listedFlowPageSchema = z.object({ items: z.array(listedFlowSchema), nextPageToken: z.string().max(1024).optional() });

export type ListedFlow = z.infer<typeof listedFlowSchema>;

export type FlowSort = '-updatedAt' | '-lastExecutionAt';

export interface OrgFlowsParams {
  projectId?: string | undefined;
  sort?: FlowSort | undefined;
  activationState?: 'ACTIVE' | 'INACTIVE' | undefined;
  /** Name prefix, case-insensitive. */
  q?: string | undefined;
}

export const orgFlowKeys = {
  list: (orgId: string, params: OrgFlowsParams) =>
    [orgId, 'flows', 'org', params.projectId ?? '', params.sort ?? '-updatedAt', params.activationState ?? '', params.q ?? ''] as const,
};

/** Lists the flows of an organization (or of one of its projects) the person can read. */
export function listOrgFlows(orgId: string, { projectId, sort, activationState, q }: OrgFlowsParams, limit = 100) {
  return apiRequest({
    method: 'GET',
    path: `/orgs/${encodeURIComponent(orgId)}/flows`,
    query: { projectId, sort, activationState, q: q === undefined || q === '' ? undefined : q, limit },
    schema: listedFlowPageSchema,
    source: 'response of GET /orgs/{orgId}/flows',
  });
}

/** The flows of the organization with their activity. Typing a search keeps the old rows until the new ones arrive. */
export function orgFlowsQueryOptions(orgId: string, params: OrgFlowsParams) {
  return queryOptions({
    queryKey: orgFlowKeys.list(orgId, params),
    queryFn: async () => (await listOrgFlows(orgId, params)).items,
    placeholderData: keepPreviousData,
  });
}

/** Switches the automatic triggers of a flow on or off. The flow must have been published; a stale tag is a 412. */
async function setFlowActive(flow: ListedFlow, active: boolean): Promise<void> {
  const action = active ? 'activate' : 'deactivate';
  await apiCall({
    method: 'POST',
    path: `/flows/${encodeURIComponent(flow.id)}/${action}`,
    ifMatch: `"${String(flow.draftRevision ?? 0)}"`,
    schema: z.object({ id: z.uuid() }),
    source: `response of POST /flows/{flowId}/${action}`,
  });
}

/**
 * Turns a flow on or off from a row. Nothing moves ahead of the server: the rows are read again after the
 * answer, and a refusal leaves the switch where it was and is shown by the screen. It is never retried.
 */
export function useSetFlowActive(orgId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationKey: [orgId, 'flows', 'activation'],
    mutationFn: ({ flow, active }: { flow: ListedFlow; active: boolean }) => setFlowActive(flow, active),
    retry: false,
    onSettled: () => queryClient.invalidateQueries({ queryKey: [orgId, 'flows'] }),
  });
}
