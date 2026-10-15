// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiRequest } from '../../../app/api/http';

/** The execution statuses and node statuses of the contract (SPEC-005 `Execution`, `NodeExecution`). */
export const executionStatuses = ['QUEUED', 'RUNNING', 'WAITING', 'SUSPENDED', 'CANCELING', 'COMPENSATING', 'SUCCEEDED', 'FAILED', 'CANCELED'] as const;
export const nodeStatuses = ['RUNNING', 'WAITING', 'SUCCEEDED', 'SUCCEEDED_WITH_ERRORS', 'FAILED', 'SUSPENDED', 'CANCELED'] as const;
export const executionModes = ['WEBHOOK', 'TRIGGER', 'TEST', 'MANUAL', 'CHILD', 'ERROR', 'RETRY', 'EVALUATION'] as const;

/** Why an execution or a node failed. The text is the server's; it is shown as given. */
const failureSchema = z.object({
  code: z.string().optional(),
  title: z.string().optional(),
  detail: z.string().optional(),
  nodeId: z.string().optional(),
});

export const executionSchema = z.looseObject({
  id: z.uuid(),
  flowId: z.uuid(),
  projectId: z.uuid(),
  flowVersion: z.number().int().min(1),
  mode: z.enum(executionModes),
  status: z.enum(executionStatuses),
  createdAt: z.string(),
  startedAt: z.string().optional(),
  finishedAt: z.string().optional(),
  retryOfExecutionId: z.uuid().optional(),
  failure: failureSchema.optional(),
  dataHidden: z.boolean(),
});

export const nodeExecutionSchema = z.looseObject({
  id: z.uuid(),
  executionId: z.uuid(),
  nodeId: z.string(),
  status: z.enum(nodeStatuses),
  attemptCount: z.number().int().min(0),
  startedAt: z.string().optional(),
  finishedAt: z.string().optional(),
  failure: failureSchema.optional(),
});

export const executionDataEntrySchema = z.looseObject({
  nodeExecutionId: z.uuid(),
  nodeId: z.string(),
  side: z.enum(['INPUT', 'OUTPUT']),
  port: z.string(),
  itemCount: z.number().int().min(0),
  dataItems: z.array(z.record(z.string(), z.unknown())),
});

const page = <T extends z.ZodType>(item: T) =>
  z.object({
    items: z.array(item),
    nextPageToken: z.string().max(1024).optional(),
  });

export const executionPageSchema = page(executionSchema);
export const nodeExecutionPageSchema = page(nodeExecutionSchema);
export const executionDataPageSchema = page(executionDataEntrySchema);

export type Execution = z.infer<typeof executionSchema>;
export type NodeExecution = z.infer<typeof nodeExecutionSchema>;
export type ExecutionDataEntry = z.infer<typeof executionDataEntrySchema>;
export type ExecutionPage = z.infer<typeof executionPageSchema>;

/** One page of the runs of a project, newest first; `flowId` keeps the runs of one flow only. */
export function listExecutions({
  projectId,
  flowId,
  pageToken,
  limit = 50,
}: {
  projectId: string;
  flowId?: string | undefined;
  pageToken?: string | undefined;
  limit?: number | undefined;
}): Promise<ExecutionPage> {
  return apiRequest({
    method: 'GET',
    path: `/projects/${encodeURIComponent(projectId)}/executions`,
    query: { flowId, pageToken, limit },
    schema: executionPageSchema,
    source: 'response of GET /projects/{projectId}/executions',
  });
}

/** One run (`getExecution`). */
export function getExecution(executionId: string): Promise<Execution> {
  return apiRequest({
    method: 'GET',
    path: `/executions/${encodeURIComponent(executionId)}`,
    schema: executionSchema,
    source: 'response of GET /executions/{executionId}',
  });
}

/** The node runs of one execution (`listExecutionNodes`). */
export async function listExecutionNodes(executionId: string): Promise<NodeExecution[]> {
  const page = await apiRequest({
    method: 'GET',
    path: `/executions/${encodeURIComponent(executionId)}/nodes`,
    query: { limit: 200 },
    schema: nodeExecutionPageSchema,
    source: 'response of GET /executions/{executionId}/nodes',
  });
  return page.items;
}

/**
 * The items each node received and produced (`listExecutionData`). It needs the permission to read
 * execution data; a person without it gets 403, which the screen shows as a message, not as an error.
 */
export async function listExecutionData(executionId: string): Promise<ExecutionDataEntry[]> {
  const page = await apiRequest({
    method: 'GET',
    path: `/executions/${encodeURIComponent(executionId)}/data`,
    query: { limit: 200 },
    schema: executionDataPageSchema,
    source: 'response of GET /executions/{executionId}/data',
  });
  return page.items;
}
