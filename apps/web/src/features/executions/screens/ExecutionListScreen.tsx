// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, Link, PageHeader, RelativeTime, StatusBadge, formatDuration } from '@orcon/ui';
import { useInfiniteQuery } from '@tanstack/react-query';
import { getRouteApi, Link as RouterLink } from '@tanstack/react-router';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { QueryState } from '../../../app/query-state/QueryState';
import { executionsInfiniteOptions } from '../api/queries';
import type { Execution } from '../api/operations';

import { modeMessages, durationOf, shortId } from './run-format';
import { runMessages } from './messages';

const messages = defineMessages({
  notRunning: { id: 'runs.list.allRuns', defaultMessage: 'Every run of this project is listed here.', description: 'Describes the runs list when it shows every run of the project.' },
});

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

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/projects/$projectHandle/executions');
  }, []);

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
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
                        {intl.formatMessage(runMessages.run)} {shortId(run.id)}
                      </RouterLink>
                    </Link>
                  </span>
                  <span>
                    <StatusBadge status={run.status} />
                  </span>
                  <span className="text-fg-muted">
                    {intl.formatMessage(runMessages.started)}:{' '}
                    {run.startedAt === undefined ? (
                      intl.formatMessage(runMessages.notStarted)
                    ) : (
                      <RelativeTime value={run.startedAt} />
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
