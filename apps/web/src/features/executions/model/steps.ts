// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { DocumentNode, Execution, NodeExecution, RunStatus } from './types';

/** Why a step failed, as the server tells it. The title and the detail are plain words; the code is the original. */
export interface StepFailure {
  code?: string | undefined;
  title?: string | undefined;
  detail?: string | undefined;
}

/** One step of a run: every run of one node of the flow, put together. */
export interface RunStep {
  /** The id of the node in the flow. */
  id: string;
  /** What the person calls it: the name in the flow, or the id when the flow is not known. */
  name: string;
  status: RunStatus;
  /** False for a node of the flow that never started in this run. */
  ran: boolean;
  startMs?: number | undefined;
  durationMs?: number | undefined;
  /** How many times the node ran (one per item of a loop, for example) and how many of those failed. */
  runs: number;
  failedRuns: number;
  /** Attempts made by all those runs, retries included. */
  attempts: number;
  failure?: StepFailure | undefined;
  /** The ids of the node runs, to find their items. */
  nodeExecutionIds: string[];
}

export interface RunTimeline {
  steps: RunStep[];
  /** The length of the run, which the bars are a share of. */
  totalMs: number;
}

/** The states in which the run is over: a node that has not started by then never will. */
const finalStatuses = new Set<Execution['status']>(['SUCCEEDED', 'FAILED', 'CANCELED']);

/** What a group of runs of one node shows as one status: the one that needs the most attention wins. */
const priority: NodeExecution['status'][] = ['FAILED', 'SUSPENDED', 'RUNNING', 'WAITING', 'SUCCEEDED_WITH_ERRORS', 'CANCELED', 'SUCCEEDED'];

function stronger(a: NodeExecution['status'], b: NodeExecution['status']): NodeExecution['status'] {
  return priority.indexOf(a) <= priority.indexOf(b) ? a : b;
}

const time = (value: string | undefined): number | undefined => {
  if (value === undefined) return undefined;
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? ms : undefined;
};

/**
 * Puts the node runs of a run on the time axis of the run: where each step started, how long it took, which steps
 * failed and which never ran. The nodes of the flow version give the names and the steps that did not run; without
 * them (the version cannot be read, or the run is of a draft) only the steps that ran are listed, named by their id.
 * A step that has not started is "did not run" only when the run is over: while it goes on, it simply has not run yet.
 */
export function buildTimeline(run: Execution, nodes: readonly NodeExecution[], documentNodes: readonly DocumentNode[] | undefined, now: number): RunTimeline {
  const names = new Map((documentNodes ?? []).map((node) => [node.id, node.name ?? node.id]));
  const groups = new Map<string, NodeExecution[]>();
  for (const node of nodes) groups.set(node.nodeId, [...(groups.get(node.nodeId) ?? []), node]);

  const starts = [time(run.startedAt), ...nodes.map((node) => time(node.startedAt))].filter((value): value is number => value !== undefined);
  const origin = starts.length === 0 ? undefined : Math.min(...starts);
  const ends = [time(run.finishedAt), ...nodes.map((node) => time(node.finishedAt))].filter((value): value is number => value !== undefined);
  const runOver = finalStatuses.has(run.status);
  const end = ends.length === 0 ? (runOver ? origin : now) : Math.max(...ends, ...(runOver ? [] : [now]));
  const totalMs = origin === undefined || end === undefined ? 0 : Math.max(0, end - origin);

  const ran: RunStep[] = [...groups.entries()].map(([id, group]) => {
    const started = group.map((node) => time(node.startedAt)).filter((value): value is number => value !== undefined);
    const finished = group.map((node) => time(node.finishedAt)).filter((value): value is number => value !== undefined);
    const first = started.length === 0 ? undefined : Math.min(...started);
    const last = finished.length === group.length && finished.length > 0 ? Math.max(...finished) : undefined;
    const failedNodes = group.filter((node) => node.status === 'FAILED');
    // The failure of the node, else the failure of the whole run when it names this node: the run keeps the plain title.
    const own = failedNodes.find((node) => node.failure !== undefined)?.failure;
    const fromRun = run.failure?.nodeId === id ? run.failure : undefined;
    const failure = own === undefined && fromRun === undefined ? undefined : { ...fromRun, ...own, title: own?.title ?? fromRun?.title };
    return {
      id,
      name: names.get(id) ?? id,
      status: group.map((node) => node.status).reduce(stronger),
      ran: true,
      startMs: first === undefined || origin === undefined ? undefined : first - origin,
      durationMs: first === undefined || last === undefined ? undefined : Math.max(0, last - first),
      runs: group.length,
      failedRuns: failedNodes.length,
      attempts: group.reduce((sum, node) => sum + node.attemptCount, 0),
      failure,
      nodeExecutionIds: group.map((node) => node.id),
    };
  });
  ran.sort((a, b) => (a.startMs ?? Number.MAX_SAFE_INTEGER) - (b.startMs ?? Number.MAX_SAFE_INTEGER));

  const notRun: RunStep[] = runOver
    ? (documentNodes ?? [])
        .filter((node) => !groups.has(node.id))
        .map((node) => ({
          id: node.id,
          name: node.name ?? node.id,
          status: 'NOT_RUN' as const,
          ran: false,
          runs: 0,
          failedRuns: 0,
          attempts: 0,
          nodeExecutionIds: [],
        }))
    : [];
  return { steps: [...ran, ...notRun], totalMs };
}
