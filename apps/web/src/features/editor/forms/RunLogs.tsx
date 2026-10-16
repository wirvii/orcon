// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query';
import { useMemo, useState, type ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { ApiProblem } from '../../../app/api/http';
import { flowQueryOptions } from '../api/queries';
import { lastRunQueryOptions, runDataQueryOptions, runNodesQueryOptions, type NodeRun } from '../api/runs';
import { LogsPanel, type LogsState, type LogStep } from '../components/LogsPanel';
import { formatAge } from '../components/SaveIndicator';
import { TestChat } from '../components/TestChat';
import { useEditorStore } from '../store/editor-context';

const between = (start: string | undefined, end: string | undefined): number | undefined => {
  if (start === undefined || end === undefined) return undefined;
  const value = Date.parse(end) - Date.parse(start);
  return Number.isFinite(value) && value >= 0 ? value : undefined;
};

const okOf = (status: NodeRun['status']): boolean | undefined => (status === 'SUCCEEDED' || status === 'SUCCEEDED_WITH_ERRORS' ? true : status === 'FAILED' ? false : undefined);

export interface RunLogsProps {
  flowId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Shows the test chat beside the output. */
  chatOpen: boolean;
  /** The link to the runs of the flow. */
  previousRuns?: ReactNode;
}

/**
 * The logs panel with its data: the last run of the flow, its steps and the output of the chosen step, read through
 * the execution contract. A server that does not answer the runs yet is "not available", never a red error; a flow
 * with no runs says so.
 */
export function RunLogs({ flowId, open, onOpenChange, chatOpen, previousRuns }: RunLogsProps) {
  const intl = useIntl();
  const names = useEditorStore((state) => state.document.nodes);
  const flow = useQuery(flowQueryOptions(flowId));
  const projectId = flow.data?.flow.projectId;
  const last = useQuery({ ...lastRunQueryOptions(projectId ?? '', flowId), enabled: projectId !== undefined });
  const runId = last.data?.id;
  const nodes = useQuery({ ...runNodesQueryOptions(flowId, runId ?? ''), enabled: runId !== undefined });
  const data = useQuery({ ...runDataQueryOptions(flowId, runId ?? ''), enabled: runId !== undefined && open });
  const [selected, setSelected] = useState<string | undefined>(undefined);

  const steps = useMemo<LogStep[]>(() => {
    const nameOf = new Map(names.map((node) => [node.id, node.name]));
    return (nodes.data ?? []).map((run) => ({
      id: run.nodeId,
      name: nameOf.get(run.nodeId) ?? run.nodeId,
      ok: okOf(run.status),
      durationMs: between(run.startedAt, run.finishedAt),
      inner: run.innerSteps?.map((step) => ({ name: step.name, durationMs: between(step.startedAt, step.finishedAt) })),
    }));
  }, [names, nodes.data]);

  const output = useMemo(() => {
    if (selected === undefined) return undefined;
    const items = (data.data ?? []).filter((entry) => entry.nodeId === selected && entry.side === 'OUTPUT').flatMap((entry) => entry.dataItems);
    return items.length === 0 ? undefined : items;
  }, [data.data, selected]);

  let state: LogsState;
  if (flow.isPending || (projectId !== undefined && last.isPending)) state = 'loading';
  else if (flow.isError || last.isError) {
    const failure = last.error ?? flow.error;
    // A server without the runs yet answers 503: that is a calm "not available", not a failure of the page.
    state = failure instanceof ApiProblem && failure.status === 503 ? 'unavailable' : 'error';
  } else if (last.data === null || last.data === undefined) state = 'empty';
  else if (nodes.isPending) state = 'loading';
  else state = 'ready';

  const run = last.data ?? undefined;
  const finished = run?.finishedAt ?? run?.startedAt ?? run?.createdAt;
  return (
    <LogsPanel
      state={state}
      open={open || chatOpen}
      onOpenChange={onOpenChange}
      run={
        run === undefined
          ? undefined
          : {
              status: run.status,
              durationMs: between(run.startedAt, run.finishedAt),
              tokens: run.usage?.totalTokens,
              when: finished === undefined ? undefined : formatAge(intl, Date.now() - Date.parse(finished)),
            }
      }
      steps={steps}
      selectedId={selected}
      onSelect={setSelected}
      output={output}
      chat={chatOpen ? <TestChat messages={[]} /> : undefined}
      previousRuns={previousRuns}
    />
  );
}
