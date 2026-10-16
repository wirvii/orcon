// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, ExecutionTimeline, PlainError, type TimelineStep } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { RotateCw } from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { problemStatus } from '../../../app/errors/to-problem-details';
import { orgFlowsQueryOptions } from '../../../app/flow-list/flows';
import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { QueryState } from '../../../app/query-state/QueryState';
import { shellProjectsQueryOptions } from '../../../app/shell/shell-data';
import { executionDataQueryOptions, executionNodesQueryOptions, executionQueryOptions, useRetryExecution, versionNodesQueryOptions } from '../api/queries';
import { ReviewPanel } from '../components/ReviewPanel';
import { RetryDialog } from '../components/RetryDialog';
import { RunHeader } from '../components/RunHeader';
import { StepPanel } from '../components/StepPanel';
import { runMessages } from '../components/messages';
import { modeMessages } from '../components/run-words';
import { durationOf, runStatusKey, tailId } from '../model/format';
import { buildTimeline, type RunStep } from '../model/steps';

export interface ExecutionDetailProps {
  orgId: string;
  executionId: string;
  /** Opens the run a retry started. */
  onRetried: (executionId: string) => void;
  /** A way back to the list, for the phone where the list and the run are on different screens. */
  back?: ReactNode;
  /** 1 when the run fills the page; 2 when it sits beside a list that has its own heading. */
  headingLevel?: 1 | 2 | undefined;
  /** The route that marks the first useful screen. */
  mark: string;
}

/** The step of the timeline of a run, as the component draws it: the remark says how many of a looping step failed. */
function toTimelineStep(step: RunStep, failedOf: (failed: number, total: number) => string): TimelineStep {
  return {
    id: step.id,
    name: step.name,
    status: step.status,
    startMs: step.startMs,
    durationMs: step.durationMs,
    note: step.runs > 1 && step.failedRuns > 0 ? failedOf(step.failedRuns, step.runs) : undefined,
  };
}

/**
 * One run: its status and the one action of that status (a retry for a failed run, the review panel for a run that
 * needs review), the timeline of its steps with how long each took and which did not run, why it failed in plain
 * words with the original error, and the items of the step that is open. Items need the permission to read run data;
 * a person without it sees the statuses and a sentence in place of the items.
 */
export function ExecutionDetail({ orgId, executionId, onRetried, back, headingLevel = 2, mark }: ExecutionDetailProps) {
  const intl = useIntl();
  const run = useQuery(executionQueryOptions(orgId, executionId));
  const nodes = useQuery(executionNodesQueryOptions(orgId, executionId));
  const current = run.data;
  // The names of the steps come from the version the run executed; without it the steps are named by their id.
  const version = useQuery({ ...versionNodesQueryOptions(orgId, current?.flowId ?? '', current?.flowVersion ?? 0), enabled: current?.flowVersion !== undefined });
  const flows = useQuery({ ...orgFlowsQueryOptions(orgId, { projectId: current?.projectId }), enabled: current !== undefined });
  const projects = useQuery(shellProjectsQueryOptions(orgId));
  // The items are asked for only when the server says they are visible: a hidden run is not read at all.
  const items = useQuery({ ...executionDataQueryOptions(orgId, executionId), enabled: current !== undefined && !current.dataHidden });
  const retry = useRetryExecution(orgId);
  const [retrying, setRetrying] = useState(false);
  const [chosen, setChosen] = useState<string | undefined>(undefined);

  useEffect(() => {
    markFirstUsefulScreen(mark);
  }, [mark]);

  const flowName = (current === undefined ? undefined : (flows.data ?? []).find((flow) => flow.id === current.flowId)?.name) ?? intl.formatMessage(runMessages.unknownFlow);
  const timeline = current === undefined || nodes.data === undefined ? undefined : buildTimeline(current, nodes.data, version.data, nodes.dataUpdatedAt);
  const firstFailed = timeline?.steps.find((step) => step.failure !== undefined);
  const selected = timeline?.steps.find((step) => step.id === (chosen ?? firstFailed?.id));
  const stepFailureShown = timeline?.steps.some((step) => step.failure !== undefined) === true;

  return (
    <div className="flex flex-col gap-6">
      <QueryState query={run} empty={{ title: intl.formatMessage(runMessages.run), description: '' }}>
        {(shown) => {
          const needsReview = shown.hasUncertainOperation === true;
          return (
            <>
              <RunHeader
                headingLevel={headingLevel}
                flowName={flowName}
                tail={tailId(shown.id)}
                status={runStatusKey(shown)}
                startedAt={shown.startedAt}
                durationMs={durationOf(shown.startedAt, shown.finishedAt)}
                mode={intl.formatMessage(modeMessages[shown.mode] ?? runMessages.modeManual)}
                version={shown.flowVersion === undefined ? intl.formatMessage(runMessages.snapshot) : String(shown.flowVersion)}
                projectName={projects.data?.find((project) => project.id === shown.projectId)?.name}
                id={shown.id}
                isRetry={shown.retryOfExecutionId !== undefined}
                back={back}
                action={
                  shown.status === 'FAILED' && !needsReview ? (
                    <Button
                      onClick={() => {
                        retry.reset();
                        setRetrying(true);
                      }}
                    >
                      <RotateCw aria-hidden className="size-icon-16" />
                      {intl.formatMessage(runMessages.retry)}
                    </Button>
                  ) : undefined
                }
              />
              {needsReview ? <ReviewPanel name={flowName} /> : null}
              {shown.failure !== undefined && !stepFailureShown ? (
                <PlainError
                  title={shown.failure.title ?? intl.formatMessage(runMessages.runFailedTitle)}
                  detail={shown.failure.detail}
                  original={shown.failure.code}
                  originalLabel={intl.formatMessage(runMessages.originalError)}
                />
              ) : null}
              <RetryDialog
                open={retrying}
                onOpenChange={setRetrying}
                version={shown.flowVersion}
                pending={retry.isPending}
                error={retry.error ?? undefined}
                onConfirm={() => {
                  retry.mutate(shown.id, {
                    onSuccess: (created) => {
                      setRetrying(false);
                      onRetried(created.id);
                    },
                  });
                }}
              />
            </>
          );
        }}
      </QueryState>

      <section aria-labelledby={`steps-${executionId}`} className="flex flex-col gap-3">
        <h2 id={`steps-${executionId}`} className="text-md font-semibold">
          {intl.formatMessage(runMessages.steps)}
        </h2>
        <QueryState query={nodes} empty={{ title: intl.formatMessage(runMessages.noSteps), description: '' }}>
          {() =>
            timeline === undefined ? null : (
              <>
                <ExecutionTimeline
                  label={intl.formatMessage(runMessages.stepsLabel)}
                  totalMs={timeline.totalMs}
                  selectedId={selected?.id}
                  onSelect={setChosen}
                  steps={timeline.steps.map((step) => toTimelineStep(step, (failed, total) => intl.formatMessage(runMessages.failedSteps, { failed, total })))}
                />
                {selected === undefined ? null : (
                  <StepPanel
                    step={selected}
                    entries={items.data}
                    dataHidden={current?.dataHidden === true || problemStatus(items.error) === 403}
                    // Query v5 reports "no error" as null, so only a failed read counts as one.
                    dataError={items.isError && problemStatus(items.error) !== 403 ? items.error : undefined}
                  />
                )}
              </>
            )
          }
        </QueryState>
      </section>
    </div>
  );
}
