// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiRequest } from '../../../app/api/http';
import {
  executionModes,
  executionStatuses,
  nodeStatuses,
  type DocumentNode,
  type Execution,
  type ExecutionDataEntry,
  type NodeExecution,
} from '../model/types';

export type { DocumentNode, Execution, ExecutionDataEntry, NodeExecution };

/** Why an execution or a node failed. The text is the server's; it is shown as given. */
const failureSchema = z.object({
  code: z.string().optional(),
  title: z.string().optional(),
  detail: z.string().optional(),
  nodeId: z.string().optional(),
});

/** The schemas are checked against the plain types the screens use: a field that drifts is a compile error. */
export const executionSchema: z.ZodType<Execution> = z.looseObject({
  id: z.uuid(),
  flowId: z.uuid(),
  projectId: z.uuid(),
  /** Absent for a run of a draft snapshot (a test run), which has no version number. */
  flowVersion: z.number().int().min(1).optional(),
  /** True when an operation of the run has an unknown outcome and a person must review it ("Needs review"). */
  hasUncertainOperation: z.boolean().optional(),
  mode: z.enum(executionModes),
  status: z.enum(executionStatuses),
  createdAt: z.string(),
  startedAt: z.string().optional(),
  finishedAt: z.string().optional(),
  retryOfExecutionId: z.uuid().optional(),
  failure: failureSchema.optional(),
  dataHidden: z.boolean(),
});

export const nodeExecutionSchema: z.ZodType<NodeExecution> = z.looseObject({
  id: z.uuid(),
  executionId: z.uuid(),
  nodeId: z.string(),
  status: z.enum(nodeStatuses),
  attemptCount: z.number().int().min(0),
  startedAt: z.string().optional(),
  finishedAt: z.string().optional(),
  failure: failureSchema.optional(),
});

export const executionDataEntrySchema: z.ZodType<ExecutionDataEntry> = z.looseObject({
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

export type ExecutionPage = z.infer<typeof executionPageSchema>;

/** The filters of the list of runs. Every one is optional; a missing one filters nothing. */
export interface ExecutionFilters {
  projectId?: string | undefined;
  flowId?: string | undefined;
  /** Only the runs that have an operation of unknown outcome ("Needs review"). */
  uncertain?: boolean | undefined;
  status?: Execution['status'] | undefined;
  /** Only the runs accepted at or after this instant. */
  createdAfter?: string | undefined;
  /** Only the run with this identifier (the search box takes an identifier only, SPEC-019 Q9). */
  id?: string | undefined;
}

/** One page of the runs of the organization the person can read, newest first (`listOrgExecutions`). */
export function listOrgExecutions({
  orgId,
  filters,
  pageToken,
  limit = 50,
}: {
  orgId: string;
  filters: ExecutionFilters;
  pageToken?: string | undefined;
  limit?: number | undefined;
}): Promise<ExecutionPage> {
  return apiRequest({
    method: 'GET',
    path: `/orgs/${encodeURIComponent(orgId)}/executions`,
    query: {
      projectId: filters.projectId,
      flowId: filters.flowId,
      status: filters.status,
      uncertain: filters.uncertain === true ? 'true' : undefined,
      createdAfter: filters.createdAfter,
      id: filters.id,
      pageToken,
      limit,
    },
    schema: executionPageSchema,
    source: 'response of GET /orgs/{orgId}/executions',
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

/** The nodes of a published version of a flow, read only for their names (the document is open and the rest is ignored). */
const versionSchema = z.looseObject({
  document: z.looseObject({ nodes: z.array(z.looseObject({ id: z.string(), name: z.string().optional() })) }),
});

/** The nodes of one published version of a flow, in the order of the document (`getFlowVersion`). */
export async function listVersionNodes(flowId: string, version: number): Promise<DocumentNode[]> {
  const answer = await apiRequest({
    method: 'GET',
    path: `/flows/${encodeURIComponent(flowId)}/versions/${String(version)}`,
    schema: versionSchema,
    source: 'response of GET /flows/{flowId}/versions/{version}',
  });
  return answer.document.nodes.map((node) => ({ id: node.id, name: node.name }));
}

/** Starts a new run that repeats a failed one on the same version of the flow (`retryExecution`). */
export function retryExecution(executionId: string): Promise<Execution> {
  return apiRequest({
    method: 'POST',
    path: `/executions/${encodeURIComponent(executionId)}/retry`,
    schema: executionSchema,
    source: 'response of POST /executions/{executionId}/retry',
  });
}
