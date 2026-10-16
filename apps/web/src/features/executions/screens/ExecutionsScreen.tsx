// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, cn, PageHeader } from '@orcon/ui';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi, Outlet, useMatch, useNavigate, useRouter } from '@tanstack/react-router';
import { RefreshCw } from 'lucide-react';
import { useEffect } from 'react';
import { useIntl } from 'react-intl';

import { orgFlowsQueryOptions } from '../../../app/flow-list/flows';
import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { QueryState } from '../../../app/query-state/QueryState';
import { shellProjectsQueryOptions } from '../../../app/shell/shell-data';
import { executionsInfiniteOptions } from '../api/queries';
import { ExecutionFilters } from '../components/ExecutionFilters';
import { ExecutionRows } from '../components/ExecutionRows';
import { runMessages } from '../components/messages';
import type { ExecutionsSearch } from '../model/params';
import type { Execution } from '../model/types';

/** The flows the flow list asks for at most (`listOrgFlows`): a shorter answer holds every flow of the scope. */
const FLOW_LIST_LIMIT = 100;

const orgRoute = getRouteApi('/_session/orgs/$orgSlug/executions');
const projectRoute = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle/executions');

interface ListPaneProps {
  orgId: string;
  search: ExecutionsSearch;
  /** The project the list is limited to, when it is the list of one project. */
  project?: { id: string; handle: string } | undefined;
  /** The run that is open beside the list. */
  selectedId?: string | undefined;
  onSearch: (patch: Partial<ExecutionsSearch>) => void;
  onClear: () => void;
  hrefOf: (run: Execution) => string;
  onOpen: (run: Execution) => void;
  /** The route that marks the first useful screen. */
  mark: string;
}

/**
 * The runs the address filters, one page at a time. It shows the filters, the rows and, under them, a button for the
 * next page. The flow of each row is named from the flows of the same scope; a run whose flow is not among them
 * belongs to a flow that was deleted, and its runs stay in the history until retention purges them.
 */
function ListPane({ orgId, search, project, selectedId, onSearch, onClear, hrefOf, onOpen, mark }: ListPaneProps) {
  const intl = useIntl();
  const effective: ExecutionsSearch = project === undefined ? search : { ...search, projectId: project.id };
  const runs = useInfiniteQuery(executionsInfiniteOptions(orgId, effective));
  const items: Execution[] | undefined = runs.data?.pages.flatMap((page) => page.items);
  const flows = useQuery(orgFlowsQueryOptions(orgId, { projectId: effective.projectId }));
  const projects = useQuery(shellProjectsQueryOptions(orgId));

  useEffect(() => {
    markFirstUsefulScreen(mark);
  }, [mark]);

  const flowNames = new Map((flows.data ?? []).map((listed) => [listed.id, listed.name]));
  const flowName = (flowId: string): string => {
    const known = flowNames.get(flowId);
    if (known !== undefined) return known;
    return flows.data !== undefined && flows.data.length < FLOW_LIST_LIMIT ? intl.formatMessage(runMessages.deletedFlow) : intl.formatMessage(runMessages.unknownFlow);
  };
  const filtering = search.show !== undefined || search.range !== undefined || search.flowId !== undefined || search.projectId !== undefined || search.q !== undefined;

  return (
    <div className="flex min-w-0 flex-col gap-3">
      <ExecutionFilters
        // A new address starts the search box over: going back to an earlier view shows that view's search.
        key={search.q ?? ''}
        search={search}
        flows={(flows.data ?? []).map((listed) => ({ value: listed.id, label: listed.name }))}
        projects={project === undefined ? (projects.data ?? []).map((listed) => ({ value: listed.id, label: listed.name })) : undefined}
        onChange={onSearch}
        onClear={onClear}
      />
      <QueryState
        query={{ status: runs.status, data: items, error: runs.error, refetch: runs.refetch }}
        unavailable={{ title: intl.formatMessage(runMessages.unavailableTitle), description: intl.formatMessage(runMessages.unavailableText) }}
        empty={
          filtering
            ? { title: intl.formatMessage(runMessages.noMatchTitle), description: intl.formatMessage(runMessages.noMatchText) }
            : { title: intl.formatMessage(runMessages.noRunsTitle), description: intl.formatMessage(runMessages.noRunsText) }
        }
      >
        {(rows) => (
          <div className="flex flex-col gap-3">
            <ExecutionRows runs={rows} flowName={flowName} hrefOf={hrefOf} onOpen={onOpen} selectedId={selectedId} />
            {runs.hasNextPage ? (
              <div>
                <Button variant="secondary" loading={runs.isFetchingNextPage} onClick={() => void runs.fetchNextPage()}>
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

/** The button that reads the list again: the list is not updated by events yet, so a person asks for the new runs. */
function RefreshButton({ orgId, search, project }: { orgId: string; search: ExecutionsSearch; project?: { id: string } | undefined }) {
  const intl = useIntl();
  const effective: ExecutionsSearch = project === undefined ? search : { ...search, projectId: project.id };
  const runs = useInfiniteQuery(executionsInfiniteOptions(orgId, effective));
  return (
    <Button variant="secondary" loading={runs.isRefetching && !runs.isFetchingNextPage} onClick={() => void runs.refetch()}>
      <RefreshCw aria-hidden className="size-icon-16" />
      {intl.formatMessage(runMessages.refresh)}
    </Button>
  );
}

/**
 * The executions of the whole organization: the list on the left with its filters in the address, and the open run on
 * the right. On a phone the list and the run are successive screens. There is no "Live" chip: the list is not
 * updated by events yet, so it has a Refresh button instead.
 */
export function OrgExecutionsScreen() {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate({ from: '/orgs/$orgSlug/executions' });
  const { organization } = orgRoute.useRouteContext();
  const { orgSlug } = orgRoute.useParams();
  const search = orgRoute.useSearch();
  const open = useMatch({ from: '/_session/orgs/$orgSlug/executions/$executionId', shouldThrow: false });
  const selectedId = open?.params.executionId;

  return (
    <div className="flex flex-col gap-4">
      {/* On a phone an open run fills the screen: its own header and a way back are all it needs above it. */}
      <div className={cn(open !== undefined && 'max-md:hidden')}>
        <PageHeader
          breadcrumb={[{ label: organization.name }, { label: intl.formatMessage(runMessages.title) }]}
          title={intl.formatMessage(runMessages.title)}
          description={intl.formatMessage(runMessages.listDescription)}
          actions={<RefreshButton orgId={organization.id} search={search} />}
        />
      </div>
      <div className="flex flex-col gap-6 md:flex-row md:items-start">
        <div className={cn('min-w-0 md:w-1/3 md:shrink-0', open !== undefined && 'max-md:hidden')}>
          <ListPane
            orgId={organization.id}
            search={search}
            selectedId={selectedId}
            mark="/orgs/$orgSlug/executions"
            onSearch={(patch) => {
              void navigate({ search: (previous) => ({ ...previous, ...patch }), replace: true });
            }}
            onClear={() => {
              void navigate({ search: {}, replace: true });
            }}
            hrefOf={(run) => router.buildLocation({ to: '/orgs/$orgSlug/executions/$executionId', params: { orgSlug, executionId: run.id }, search }).href}
            onOpen={(run) => {
              void navigate({ to: '/orgs/$orgSlug/executions/$executionId', params: { orgSlug, executionId: run.id }, search });
            }}
          />
        </div>
        <div className={cn('min-w-0 md:flex-1', open === undefined && 'max-md:hidden')}>
          <Outlet />
        </div>
      </div>
    </div>
  );
}

/** What the right side says while no run is open. */
export function PickAnExecution() {
  const intl = useIntl();
  return <p className="text-fg-muted border-border rounded-lg border border-dashed p-6">{intl.formatMessage(runMessages.pickOne)}</p>;
}

/**
 * The runs of one project, as the Executions tab of the project: the same list and filters, limited to the project.
 * A row opens the run on its own page, because the tab is already inside the page of the project.
 */
export function ProjectExecutionsScreen() {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate({ from: '/orgs/$orgSlug/projects/$projectHandle/executions' });
  const { organization, project } = projectRoute.useRouteContext();
  const { orgSlug, projectHandle } = projectRoute.useParams();
  const search = projectRoute.useSearch();

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        headingLevel={2}
        title={intl.formatMessage(runMessages.title)}
        description={intl.formatMessage(runMessages.projectDescription)}
        actions={<RefreshButton orgId={organization.id} search={search} project={project} />}
      />
      <ListPane
        orgId={organization.id}
        search={search}
        project={project}
        mark="/orgs/$orgSlug/projects/$projectHandle/executions"
        onSearch={(patch) => {
          void navigate({ search: (previous) => ({ ...previous, ...patch }), replace: true });
        }}
        onClear={() => {
          void navigate({ search: {}, replace: true });
        }}
        hrefOf={(run) =>
          router.buildLocation({ to: '/orgs/$orgSlug/projects/$projectHandle/executions/$executionId', params: { orgSlug, projectHandle, executionId: run.id }, search }).href
        }
        onOpen={(run) => {
          void navigate({ to: '/orgs/$orgSlug/projects/$projectHandle/executions/$executionId', params: { orgSlug, projectHandle, executionId: run.id }, search });
        }}
      />
    </div>
  );
}
