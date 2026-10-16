// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, DateTime, Link, PageHeader, RelativeTime, StatusBadge, formatDuration } from '@orcon/ui';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi, Link as RouterLink } from '@tanstack/react-router';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { orgFlowsQueryOptions } from '../../../app/flow-list/flows';
import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { QueryState } from '../../../app/query-state/QueryState';
import { executionsInfiniteOptions } from '../api/queries';
import type { Execution } from '../api/operations';

import { modeMessages, durationOf, tailId } from './run-format';
import { runMessages } from './messages';

const messages = defineMessages({
  notRunning: { id: 'runs.list.allRuns', defaultMessage: 'Every run of this project is listed here.', description: 'Describes the runs list when it shows every run of the project.' },
});

/** The flows the project list asks for at most (`listOrgFlows`): a shorter answer holds every flow of the project. */
const FLOW_LIST_LIMIT = 100;

const projectRoute = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle');
const listRoute = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle/executions');

/**
 * The runs of the project, newest first, one page at a time. The flow filter comes from the address, so
 * the list of a flow is a link. A run's short identifier opens its detail.
 */
export function ExecutionListScreen() {
  const intl = useIntl();
  const { organization, project } = projectRoute.useRouteContext();
  const { orgSlug, projectHandle } = projectRoute.useParams();
  const { flowId } = listRoute.useSearch();
  const runs = useInfiniteQuery(executionsInfiniteOptions(organization.id, project.id, flowId));
  const items: Execution[] | undefined = runs.data?.pages.flatMap((page) => page.items);
  // The names of the flows of the project. A run whose flow is not among them belongs to a flow that was deleted:
  // its runs stay in the history until retention purges the flow, so the row says so instead of hiding the run.
  const flows = useQuery(orgFlowsQueryOptions(organization.id, { projectId: project.id }));
  const flowNames = new Map((flows.data ?? []).map((listed) => [listed.id, listed.name]));
  const flowName = (flowId: string): string => {
    const known = flowNames.get(flowId);
    if (known !== undefined) return known;
    return flows.data !== undefined && flows.data.length < FLOW_LIST_LIMIT ? intl.formatMessage(runMessages.deletedFlow) : intl.formatMessage(runMessages.unknownFlow);
  };

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/projects/$projectHandle/executions');
  }, []);

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        headingLevel={2}
        title={intl.formatMessage(runMessages.title)}
        description={intl.formatMessage(flowId === undefined ? messages.notRunning : runMessages.titleOfFlow)}
        actions={
          flowId === undefined ? undefined : (
            <Link asChild>
              <RouterLink to="/orgs/$orgSlug/projects/$projectHandle/executions" params={{ orgSlug, projectHandle }}>
                {intl.formatMessage(runMessages.showAll)}
              </RouterLink>
            </Link>
          )
        }
      />
      <QueryState
        query={{ status: runs.status, data: items, error: runs.error, refetch: runs.refetch }}
        unavailable={{ title: intl.formatMessage(runMessages.unavailableTitle), description: intl.formatMessage(runMessages.unavailableText) }}
        empty={{ title: intl.formatMessage(runMessages.noRunsTitle), description: intl.formatMessage(runMessages.noRunsText) }}
      >
        {(rows) => (
          <div className="flex flex-col gap-3">
            <ul className="border-border flex flex-col rounded-lg border">
              {rows.map((run) => (
                <li key={run.id} className="border-border flex flex-col gap-1 border-b p-4 last:border-b-0 md:grid md:grid-cols-4 md:items-center md:gap-4">
                  <span className="font-medium">
                    <Link asChild>
                      <RouterLink
                        to="/orgs/$orgSlug/projects/$projectHandle/executions/$executionId"
                        params={{ orgSlug, projectHandle, executionId: run.id }}
                      >
                        {flowName(run.flowId)}
                      </RouterLink>
                    </Link>
                    <small className="text-fg-muted block font-normal">
                      {intl.formatMessage(runMessages.run)} #{tailId(run.id)}
                    </small>
                  </span>
                  <span>
                    <StatusBadge status={run.status} />
                  </span>
                  <span className="text-fg-muted">
                    {intl.formatMessage(runMessages.started)}:{' '}
                    {run.startedAt === undefined ? (
                      intl.formatMessage(runMessages.notStarted)
                    ) : (
                      <>
                        <DateTime value={run.startedAt} dateStyle="medium" timeStyle="medium" /> (<RelativeTime value={run.startedAt} />)
                      </>
                    )}
                  </span>
                  <span className="text-fg-muted">
                    {intl.formatMessage(runMessages.duration)}:{' '}
                    {(() => {
                      const ms = durationOf(run.startedAt, run.finishedAt);
                      return ms === undefined ? intl.formatMessage(runMessages.stillRunning) : formatDuration(ms, intl.locale);
                    })()}
                  </span>
                  <span className="text-fg-muted md:col-span-4">
                    {intl.formatMessage(runMessages.trigger)}: {intl.formatMessage(modeMessages[run.mode] ?? runMessages.modeManual)}
                  </span>
                </li>
              ))}
            </ul>
            {runs.hasNextPage ? (
              <div>
                <Button
                  variant="secondary"
                  loading={runs.isFetchingNextPage}
                  onClick={() => {
                    void runs.fetchNextPage();
                  }}
                >
                  {intl.formatMessage(runMessages.loadMore)}
                </Button>
              </div>
            ) : null}
          </div>
        )}
      </QueryState>
    </div>
  );
}
