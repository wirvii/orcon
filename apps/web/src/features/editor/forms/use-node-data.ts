// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { PaneData } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

import { flowQueryOptions } from '../api/queries';
import { lastRunQueryOptions, runDataQueryOptions, type RunData } from '../api/runs';

/** The input and output of one node in the last run of the flow. */
export interface NodeData {
  /** What the node received, when the last run recorded it. */
  input: PaneData | undefined;
  /** What the node produced, one entry per output port. */
  outputs: readonly { port: string; data: PaneData }[];
  status: 'loading' | 'ready' | 'none';
}

const toPane = (entries: readonly RunData[], from: string | undefined): PaneData | undefined => {
  if (entries.length === 0) return undefined;
  return { items: entries.flatMap((entry) => entry.dataItems), from };
};

/**
 * The data the last run of the flow recorded for a node (SPEC-014 R-ED-EXPR-06: fields come from the latest run). It is read
 * through the execution contract; a flow that never ran, a server that does not answer or a person who may not read execution
 * data has none, and the panes then show their empty state. `from` names the step the input came from.
 */
export function useNodeData(flowId: string | undefined, nodeId: string, from: string | undefined): NodeData {
  const flow = useQuery({ ...flowQueryOptions(flowId ?? ''), enabled: flowId !== undefined });
  const projectId = flow.data?.flow.projectId;
  const last = useQuery({ ...lastRunQueryOptions(projectId ?? '', flowId ?? ''), enabled: projectId !== undefined });
  const runId = last.data?.id;
  const data = useQuery({ ...runDataQueryOptions(flowId ?? '', runId ?? ''), enabled: runId !== undefined });
  return useMemo<NodeData>(() => {
    if (flowId === undefined || data.data === undefined) {
      return { input: undefined, outputs: [], status: flowId !== undefined && (flow.isPending || last.isPending || (runId !== undefined && data.isPending)) ? 'loading' : 'none' };
    }
    const mine = data.data.filter((entry) => entry.nodeId === nodeId);
    const ports = Array.from(new Set(mine.filter((entry) => entry.side === 'OUTPUT').map((entry) => entry.port)));
    return {
      input: toPane(
        mine.filter((entry) => entry.side === 'INPUT'),
        from,
      ),
      outputs: ports.flatMap((port) => {
        const pane = toPane(
          mine.filter((entry) => entry.side === 'OUTPUT' && entry.port === port),
          undefined,
        );
        return pane ? [{ port, data: pane }] : [];
      }),
      status: 'ready',
    };
  }, [data.data, data.isPending, flow.isPending, flowId, from, last.isPending, nodeId, runId]);
}
