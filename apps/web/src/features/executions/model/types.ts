// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The shapes the executions screens read. The API layer parses the answers of the server into these (its schemas are
 * checked against them at compile time), so the model and the components work on plain types.
 */

export const executionStatuses = ['QUEUED', 'RUNNING', 'WAITING', 'SUSPENDED', 'CANCELING', 'COMPENSATING', 'SUCCEEDED', 'FAILED', 'CANCELED'] as const;
export const nodeStatuses = ['RUNNING', 'WAITING', 'SUCCEEDED', 'SUCCEEDED_WITH_ERRORS', 'FAILED', 'SUSPENDED', 'CANCELED'] as const;
export const executionModes = ['WEBHOOK', 'TRIGGER', 'TEST', 'MANUAL', 'CHILD', 'ERROR', 'RETRY', 'EVALUATION'] as const;

export type ExecutionStatus = (typeof executionStatuses)[number];
export type NodeStatus = (typeof nodeStatuses)[number];
export type ExecutionMode = (typeof executionModes)[number];

/** Every state a run or a step is shown with: the engine statuses, plus "needs review" and "did not run". */
export type RunStatus = ExecutionStatus | NodeStatus | 'UNCERTAIN' | 'NOT_RUN';

/** Why an execution or a node failed. The text is the server's; it is shown as given. */
export interface Failure {
  code?: string | undefined;
  title?: string | undefined;
  detail?: string | undefined;
  nodeId?: string | undefined;
}

export interface Execution {
  id: string;
  flowId: string;
  projectId: string;
  /** Absent for a run of a draft snapshot (a test run), which has no version number. */
  flowVersion?: number | undefined;
  /** True when an operation of the run has an unknown outcome and a person must review it ("Needs review"). */
  hasUncertainOperation?: boolean | undefined;
  mode: ExecutionMode;
  status: ExecutionStatus;
  createdAt: string;
  startedAt?: string | undefined;
  finishedAt?: string | undefined;
  retryOfExecutionId?: string | undefined;
  failure?: Failure | undefined;
  dataHidden: boolean;
}

export interface NodeExecution {
  id: string;
  executionId: string;
  nodeId: string;
  status: NodeStatus;
  attemptCount: number;
  startedAt?: string | undefined;
  finishedAt?: string | undefined;
  failure?: Failure | undefined;
}

export interface ExecutionDataEntry {
  nodeExecutionId: string;
  nodeId: string;
  side: 'INPUT' | 'OUTPUT';
  port: string;
  itemCount: number;
  dataItems: Record<string, unknown>[];
}

/** A node of a published version of a flow: its id and the name the person gave it. */
export interface DocumentNode {
  id: string;
  name: string | undefined;
}
