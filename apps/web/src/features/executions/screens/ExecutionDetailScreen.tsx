// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, CodeBlock, PageHeader, ProblemAlert, RelativeTime, StatusBadge, formatDuration } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useEffect } from 'react';
import { useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { problemStatus, toProblemDetails } from '../../../app/errors/to-problem-details';
import { QueryState } from '../../../app/query-state/QueryState';
import { executionDataQueryOptions, executionNodesQueryOptions, executionQueryOptions } from '../api/queries';
import type { ExecutionDataEntry, NodeExecution } from '../api/operations';

import { runMessages } from './messages';
import { durationOf, modeMessages, shortId } from './run-format';

const detailRoute = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle/executions/$executionId');
const projectRoute = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle');

/**
 * One run: its status, why it failed if it did, and each step with the items it received and produced.
 * Items need the permission to read run data; a person without it sees the statuses and a message.
 */
export function ExecutionDetailScreen() {
  const intl = useIntl();
  const { organization } = projectRoute.useRouteContext();
  const { executionId } = detailRoute.useParams();
  const run = useQuery(executionQueryOptions(organization.id, executionId));
  const nodes = useQuery(executionNodesQueryOptions(organization.id, executionId));
  // The items are asked for only when the server says they are visible: a hidden run is not read at all.
  const items = useQuery({
    ...executionDataQueryOptions(organization.id, executionId),
    enabled: run.data !== undefined && !run.data.dataHidden,
  });

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/projects/$projectHandle/executions/$executionId');
  }, []);

  return (
    <div className="flex flex-col gap-4">
      <QueryState query={run} empty={{ title: intl.formatMessage(runMessages.run), description: '' }}>
        {(current) => {
          const ms = durationOf(current.startedAt, current.finishedAt);
          return (
            <>
              <PageHeader
                title={`${intl.formatMessage(runMessages.run)} ${shortId(current.id)}`}
                description={`${intl.formatMessage(modeMessages[current.mode] ?? runMessages.modeManual)} · ${intl.formatMessage(runMessages.version, { version: current.flowVersion })}`}
              />
              <div className="flex flex-wrap items-center gap-3">
                <StatusBadge status={current.status} />
                <span className="text-fg-muted">
                  {intl.formatMessage(runMessages.started)}:{' '}
                  {current.startedAt === undefined ? intl.formatMessage(runMessages.notStarted) : <RelativeTime value={current.startedAt} />}
                </span>
                <span className="text-fg-muted">
                  {intl.formatMessage(runMessages.duration)}:{' '}
                  {ms === undefined ? intl.formatMessage(runMessages.stillRunning) : formatDuration(ms, intl.locale)}
                </span>
              </div>
              {current.failure ? (
                <Banner tone="danger" title={current.failure.title ?? intl.formatMessage(runMessages.run)}>
                  <div className="flex flex-col gap-1">
                    {intl.formatMessage(runMessages.failedTitle)}
                    {current.failure.detail === undefined ? null : <span>{current.failure.detail}</span>}
                    {current.failure.nodeId === undefined ? null : (
                      <span>{intl.formatMessage(runMessages.failedNode, { node: current.failure.nodeId })}</span>
                    )}
                  </div>
                </Banner>
              ) : null}
              {current.dataHidden ? <Banner tone="info" title={intl.formatMessage(runMessages.dataHidden)} /> : null}
            </>
          );
        }}
      </QueryState>

      <section aria-labelledby="run-steps" className="flex flex-col gap-3">
        <h2 id="run-steps" className="text-lg font-semibold">
          {intl.formatMessage(runMessages.nodes)}
        </h2>
        <QueryState query={nodes} empty={{ title: intl.formatMessage(runMessages.noNodes), description: '' }}>
          {(steps) => (
            <ol className="flex flex-col gap-3">
              {steps.map((step) => (
                <StepView
                  key={step.id}
                  step={step}
                  entries={items.data}
                  // Query v5 reports "no error" as null, so only a failed read counts as one.
                  dataError={items.isError ? items.error : undefined}
                  dataHidden={run.data?.dataHidden === true}
                />
              ))}
            </ol>
          )}
        </QueryState>
      </section>
    </div>
  );
}

interface StepViewProps {
  step: NodeExecution;
  entries: ExecutionDataEntry[] | undefined;
  dataError: unknown;
  dataHidden: boolean;
}

/** One step of the run: its status, its failure, and the items it received and produced. */
function StepView({ step, entries, dataError, dataHidden }: StepViewProps) {
  const intl = useIntl();
  const ms = durationOf(step.startedAt, step.finishedAt);
  const mine = (entries ?? []).filter((entry) => entry.nodeExecutionId === step.id);
  const forbidden = problemStatus(dataError) === 403;

  return (
    <li className="border-border flex flex-col gap-3 rounded-lg border p-4">
      <div className="flex flex-wrap items-center gap-3">
        <h3 className="font-semibold wrap-anywhere">{step.nodeId}</h3>
        <StatusBadge status={step.status} />
        <span className="text-fg-muted">{intl.formatMessage(runMessages.attempts, { count: step.attemptCount })}</span>
        <span className="text-fg-muted">
          {intl.formatMessage(runMessages.duration)}: {ms === undefined ? intl.formatMessage(runMessages.stillRunning) : formatDuration(ms, intl.locale)}
        </span>
      </div>
      {step.failure?.detail === undefined ? null : <p className="text-status-danger wrap-anywhere">{step.failure.detail}</p>}

      {dataHidden || forbidden ? (
        <p className="text-fg-muted">{intl.formatMessage(runMessages.dataHidden)}</p>
      ) : dataError !== undefined ? (
        <ProblemAlert problem={toProblemDetails(dataError)} />
      ) : (
        ['INPUT', 'OUTPUT'].map((side) => {
          const port = mine.filter((entry) => entry.side === side);
          if (port.length === 0) return null;
          return port.map((entry) => (
            <CodeBlock
              key={`${entry.side}-${entry.port}`}
              label={
                side === 'INPUT'
                  ? intl.formatMessage(runMessages.input, { node: step.nodeId, port: entry.port })
                  : intl.formatMessage(runMessages.output, { node: step.nodeId, port: entry.port })
              }
              code={entry.dataItems.length === 0 ? intl.formatMessage(runMessages.noItems) : JSON.stringify(entry.dataItems, null, 2)}
            />
          ));
        })
      )}
    </li>
  );
}
