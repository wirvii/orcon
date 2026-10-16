// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { apiRequest } from '../../../app/api/http';

import { editorKeys, LOCAL_ORGANIZATION } from './queries';

/**
 * The runs of the open flow, for the logs panel (SPEC-019 R-UX-EDT-06). These are reads of the execution contract
 * (`listExecutions`, `listExecutionNodes`, `listExecutionData`); the panel shows what they answer and invents nothing.
 * Token counts are read when the answer has them (`usage`) and left out when it does not.
 */

const failureSchema = z.looseObject({ code: z.string().optional(), title: z.string().optional(), detail: z.string().optional() });

const usageSchema = z.looseObject({ totalTokens: z.number().int().min(0).optional() });

export const runSchema = z.looseObject({
  id: z.uuid(),
  flowId: z.uuid(),
  status: z.enum(['QUEUED', 'RUNNING', 'WAITING', 'SUSPENDED', 'CANCELING', 'COMPENSATING', 'SUCCEEDED', 'FAILED', 'CANCELED']),
  createdAt: z.string(),
  startedAt: z.string().optional(),
  finishedAt: z.string().optional(),
  failure: failureSchema.optional(),
  usage: usageSchema.optional(),
});

export const nodeRunSchema = z.looseObject({
  id: z.uuid(),
  nodeId: z.string(),
  status: z.enum(['RUNNING', 'WAITING', 'SUCCEEDED', 'SUCCEEDED_WITH_ERRORS', 'FAILED', 'SUSPENDED', 'CANCELED']),
  startedAt: z.string().optional(),
  finishedAt: z.string().optional(),
  failure: failureSchema.optional(),
  /** The steps inside an agent step (memory, tools, model calls), when the execution records them. */
  innerSteps: z.array(z.looseObject({ name: z.string(), startedAt: z.string().optional(), finishedAt: z.string().optional() })).optional(),
});

export const runDataSchema = z.looseObject({
  nodeId: z.string(),
  side: z.enum(['INPUT', 'OUTPUT']),
  port: z.string(),
  itemCount: z.number().int().min(0),
  dataItems: z.array(z.record(z.string(), z.unknown())),
});

export type Run = z.infer<typeof runSchema>;
export type NodeRun = z.infer<typeof nodeRunSchema>;
export type RunData = z.infer<typeof runDataSchema>;

const page = <T extends z.ZodType>(item: T) => z.object({ items: z.array(item), nextPageToken: z.string().optional() });

/** The newest run of one flow, or none. */
async function lastRun(projectId: string, flowId: string): Promise<Run | undefined> {
  const answer = await apiRequest({
    method: 'GET',
    path: `/projects/${encodeURIComponent(projectId)}/executions`,
    query: { flowId, limit: 1 },
    schema: page(runSchema),
    source: 'response of GET /projects/{projectId}/executions',
  });
  return answer.items[0];
}

async function runNodes(executionId: string): Promise<NodeRun[]> {
  const answer = await apiRequest({
    method: 'GET',
    path: `/executions/${encodeURIComponent(executionId)}/nodes`,
    query: { limit: 200 },
    schema: page(nodeRunSchema),
    source: 'response of GET /executions/{executionId}/nodes',
  });
  return answer.items;
}

async function runData(executionId: string): Promise<RunData[]> {
  const answer = await apiRequest({
    method: 'GET',
    path: `/executions/${encodeURIComponent(executionId)}/data`,
    query: { limit: 200 },
    schema: page(runDataSchema),
    source: 'response of GET /executions/{executionId}/data',
  });
  return answer.items;
}

export const lastRunQueryOptions = (projectId: string, flowId: string) =>
  queryOptions({
    queryKey: editorKeys.runs(LOCAL_ORGANIZATION, flowId, 'last', projectId),
    queryFn: () => lastRun(projectId, flowId),
    staleTime: 10_000,
    retry: false,
  });

export const runNodesQueryOptions = (flowId: string, executionId: string) =>
  queryOptions({
    queryKey: editorKeys.runs(LOCAL_ORGANIZATION, flowId, 'nodes', executionId),
    queryFn: () => runNodes(executionId),
    staleTime: 30_000,
    retry: false,
  });

export const runDataQueryOptions = (flowId: string, executionId: string) =>
  queryOptions({
    queryKey: editorKeys.runs(LOCAL_ORGANIZATION, flowId, 'data', executionId),
    queryFn: () => runData(executionId),
    staleTime: 30_000,
    retry: false,
  });
