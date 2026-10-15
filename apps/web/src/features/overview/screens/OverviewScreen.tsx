// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Button,
  ComingSoon,
  CountTabs,
  FilterChip,
  Input,
  Mascot,
  PageHeader,
  ProblemAlert,
  Select,
  Skeleton,
  type CountTab,
} from '@orcon/ui';
import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { getRouteApi, useNavigate, useRouter } from '@tanstack/react-router';
import { KeyRound, Activity as ActivityIcon, Plus, Search, Upload } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { toProblemDetails } from '../../../app/errors/to-problem-details';
import { FlowRows } from '../../../app/flow-list/FlowRows';
import { orgFlowsQueryOptions, useSetFlowActive, type FlowSort } from '../../../app/flow-list/flows';
import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { QueryState } from '../../../app/query-state/QueryState';
import { currentUserQueryOptions } from '../../../app/session/current-user';
import { identityOf } from '../../../app/session/identity';
import { shellProjectsQueryOptions } from '../../../app/shell/shell-data';
import type { ActivitySummary } from '../api/operations';
import { activitySummaryQueryOptions } from '../api/queries';
import { ActivityStats } from '../components/ActivityStats';
import { greetingFor } from '../model/greeting';
import type { OverviewTab } from '../model/params';

const messages = defineMessages({
  morning: { id: 'overview.greeting.morning', defaultMessage: 'Good morning, {name}', description: 'Heading of the home page before noon. {name} is the first name of the person.' },
  afternoon: { id: 'overview.greeting.afternoon', defaultMessage: 'Good afternoon, {name}', description: 'Heading of the home page from noon until the evening. {name} is the first name of the person.' },
  evening: { id: 'overview.greeting.evening', defaultMessage: 'Good evening, {name}', description: 'Heading of the home page in the evening and at night. {name} is the first name of the person.' },
  description: {
    id: 'overview.description',
    defaultMessage: 'Everything in {organization}, from all the projects you can see, in one place.',
    description: 'One line under the heading of the home page. {organization} is the name of the organization.',
  },
  home: { id: 'overview.home', defaultMessage: 'Home', description: 'Last step of the breadcrumb of the home page.' },
  createFlow: { id: 'overview.createFlow', defaultMessage: 'Create flow', description: 'Button on the home page that starts a new flow.' },
  importN8n: { id: 'overview.importN8n', defaultMessage: 'Import from n8n', description: 'Button on the home page that starts a flow from an n8n workflow.' },
  tabs: { id: 'overview.tabs', defaultMessage: 'What to show', description: 'Accessible name of the tabs of the home page: flows, credentials and executions.' },
  tabFlows: { id: 'overview.tab.flows', defaultMessage: 'Flows', description: 'Tab of the home page that lists the flows of every project.' },
  tabCredentials: { id: 'overview.tab.credentials', defaultMessage: 'Credentials', description: 'Tab of the home page that lists the credentials of every project.' },
  tabExecutions: { id: 'overview.tab.executions', defaultMessage: 'Executions', description: 'Tab of the home page that lists the executions of every project.' },
  search: { id: 'overview.search', defaultMessage: 'Search flows by name', description: 'Accessible name and placeholder of the search box of the flows list.' },
  filterState: { id: 'overview.filter.state', defaultMessage: 'State', description: 'Name of the filter of the flows list by whether the flow is active.' },
  filterActive: { id: 'overview.filter.active', defaultMessage: 'Active', description: 'Choice of the state filter that keeps only the active flows.' },
  filterInactive: { id: 'overview.filter.inactive', defaultMessage: 'Inactive', description: 'Choice of the state filter that keeps only the inactive flows.' },
  sort: { id: 'overview.sort', defaultMessage: 'Order', description: 'Accessible name of the choice of the order of the flows list.' },
  sortUpdated: { id: 'overview.sort.updated', defaultMessage: 'Last change', description: 'Order of the flows list by when the flow last changed.' },
  sortLastRun: { id: 'overview.sort.lastRun', defaultMessage: 'Last run', description: 'Order of the flows list by when the flow last ran.' },
  flows: { id: 'overview.flows.heading', defaultMessage: 'Flows', description: 'Heading of the list of flows on the home page.' },
  emptyTitle: { id: 'overview.empty.title', defaultMessage: 'No flows yet', description: 'Title of the home page list when no project has a flow.' },
  emptyText: {
    id: 'overview.empty.text',
    defaultMessage: 'Create your first flow. It starts inactive: nothing runs until you publish it.',
    description: 'Explains the empty list of flows on the home page and the first step.',
  },
  noMatchTitle: { id: 'overview.noMatch.title', defaultMessage: 'No flow matches', description: 'Title of the flows list when the search or the filters leave nothing.' },
  noMatchText: { id: 'overview.noMatch.text', defaultMessage: 'Change the search or clear the filters to see more flows.', description: 'Says what to do when the search or the filters leave no flow.' },
  noProjectsTitle: { id: 'overview.noProjects.title', defaultMessage: 'No projects yet', description: 'Title of the home page when the organization has no project the person can read.' },
  noProjectsText: {
    id: 'overview.noProjects.text',
    defaultMessage: 'A project groups the flows of one area of work. Create one to start.',
    description: 'Explains the empty home page when there is no project and what to do.',
  },
  createProject: { id: 'overview.createProject', defaultMessage: 'Create a project', description: 'Button on the home page that opens the new project dialog.' },
  loading: { id: 'overview.loading', defaultMessage: 'Loading flows', description: 'Accessible name of the placeholders shown while the flows of the home page load.' },
  loadingFigures: { id: 'overview.loadingFigures', defaultMessage: 'Loading the figures', description: 'Accessible name of the placeholders shown while the five figures of the home page load.' },
  credentialsSoonTitle: { id: 'overview.credentials.soon.title', defaultMessage: 'Credentials of every project', description: 'Title of the home page tab of credentials, which is not available yet.' },
  credentialsSoonText: {
    id: 'overview.credentials.soon.text',
    defaultMessage: 'One list for the whole organization is not ready yet. Open a project to see and manage its credentials.',
    description: 'Says that the credentials of the whole organization cannot be listed yet and where to find them.',
  },
  executionsSoonTitle: { id: 'overview.executions.soon.title', defaultMessage: 'Executions of every project', description: 'Title of the home page tab of executions, which is not available yet.' },
  executionsSoonText: {
    id: 'overview.executions.soon.text',
    defaultMessage: 'One list for the whole organization is not ready yet. Open a project to see its executions.',
    description: 'Says that the executions of the whole organization cannot be listed yet and where to find them.',
  },
});

const route = getRouteApi('/_session/orgs/$orgSlug/overview');

/**
 * Home of an organization: a greeting, five figures of the activity, and the flows of every project the person can
 * read with their last execution and the Active switch. The figures appear only when the server can compute them;
 * without them the page shows no zeros (SPEC-019 UX3). Credentials and executions of the whole organization have
 * no endpoint yet, so their tabs say so.
 */
export function OverviewScreen() {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate();
  const { organization } = route.useRouteContext();
  const { orgSlug } = route.useParams();
  const search = route.useSearch();
  const tab: OverviewTab = search.tab ?? 'flows';
  const user = useQuery(currentUserQueryOptions());
  const projects = useQuery(shellProjectsQueryOptions(organization.id));
  const summary = useQuery(activitySummaryQueryOptions(organization.id));

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/overview');
  }, []);

  const firstName = user.data === undefined ? '' : (identityOf(user.data).name.split(/\s+/)[0] ?? '');
  const greeting = { morning: messages.morning, afternoon: messages.afternoon, evening: messages.evening }[greetingFor(new Date().getHours())];

  const createFlow = (how?: 'import') => {
    const first = projects.data?.[0];
    if (first === undefined) {
      void navigate({ to: '/orgs/$orgSlug/projects', params: { orgSlug }, search: { create: true } });
    } else {
      void navigate({ to: '/orgs/$orgSlug/projects/$projectHandle/flows', params: { orgSlug, projectHandle: first.handle }, search: { create: how ?? true } });
    }
  };

  const tabs: CountTab[] = (['flows', 'credentials', 'executions'] as const).map((id) => ({
    id,
    label: intl.formatMessage({ flows: messages.tabFlows, credentials: messages.tabCredentials, executions: messages.tabExecutions }[id]),
    count: id === 'flows' ? summary.data?.flowCount : undefined,
    href: router.buildLocation({ to: '/orgs/$orgSlug/overview', params: { orgSlug }, search: { tab: id === 'flows' ? undefined : id } }).href,
  }));

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        breadcrumb={[{ label: organization.name }, { label: intl.formatMessage(messages.home) }]}
        title={intl.formatMessage(greeting, { name: firstName })}
        description={intl.formatMessage(messages.description, { organization: organization.name })}
        actions={
          <>
            <Button
              variant="secondary"
              className="max-md:hidden"
              onClick={() => {
                createFlow('import');
              }}
            >
              <Upload aria-hidden className="size-icon-16" />
              {intl.formatMessage(messages.importN8n)}
            </Button>
            <Button
              onClick={() => {
                createFlow();
              }}
            >
              <Plus aria-hidden className="size-icon-16" />
              {intl.formatMessage(messages.createFlow)}
            </Button>
          </>
        }
      />
      <Figures query={summary} />
      <QueryState
        query={projects}
        empty={{
          title: intl.formatMessage(messages.noProjectsTitle),
          description: intl.formatMessage(messages.noProjectsText),
          action: (
            <Button
              onClick={() => {
                createFlow();
              }}
            >
              {intl.formatMessage(messages.createProject)}
            </Button>
          ),
          illustration: <Mascot pose="thinking" />,
        }}
      >
        {(items) => (
          <section aria-labelledby="overview-tabs" className="flex flex-col gap-3">
            <h2 id="overview-tabs" className="sr-only">
              {intl.formatMessage(messages.flows)}
            </h2>
            <CountTabs
              tabs={tabs}
              currentId={tab}
              label={intl.formatMessage(messages.tabs)}
              onNavigate={(next) => {
                void navigate({ href: next.href });
              }}
            />
            {tab === 'flows' ? (
              <FlowsTab
                orgId={organization.id}
                orgSlug={orgSlug}
                projects={items}
                onCreate={() => {
                  createFlow();
                }}
              />
            ) : tab === 'credentials' ? (
              <ComingSoon icon={KeyRound} title={intl.formatMessage(messages.credentialsSoonTitle)} description={intl.formatMessage(messages.credentialsSoonText)} />
            ) : (
              <ComingSoon icon={ActivityIcon} title={intl.formatMessage(messages.executionsSoonTitle)} description={intl.formatMessage(messages.executionsSoonText)} />
            )}
          </section>
        )}
      </QueryState>
    </div>
  );
}

/**
 * The five figures. While they load there are placeholders; when the server says they are not available (503) or
 * cannot compute them, the row is not drawn at all: a zero the product does not know is worse than no number.
 */
function Figures({ query }: { query: UseQueryResult<ActivitySummary> }) {
  const intl = useIntl();
  if (query.status === 'pending') {
    return (
      <div role="status" aria-busy="true" aria-label={intl.formatMessage(messages.loadingFigures)} className="grid grid-cols-2 gap-2 md:grid-cols-5 md:gap-3">
        {[0, 1, 2, 3, 4].map((card) => (
          <Skeleton key={card} className="h-16 w-full" />
        ))}
      </div>
    );
  }
  if (query.status === 'error') {
    const problem = toProblemDetails(query.error);
    // Not available yet, or no permission to read the figures: the page simply has no figures.
    if (problem.code === 'service-unavailable' || problem.status === 403 || problem.status === 503) return null;
    return <ProblemAlert problem={problem} />;
  }
  return <ActivityStats figures={query.data} />;
}

interface FlowsTabProps {
  orgId: string;
  orgSlug: string;
  projects: readonly { id: string; name: string; handle: string }[];
  onCreate: () => void;
}

function FlowsTab({ orgId, orgSlug, projects, onCreate }: FlowsTabProps) {
  const intl = useIntl();
  const navigate = useNavigate();
  const search = route.useSearch();
  const sort: FlowSort = search.sort ?? '-updatedAt';
  const [text, setText] = useState(search.q ?? '');

  // The search goes to the address after a short pause, so the list is not read on every key.
  useEffect(() => {
    if (text === (search.q ?? '')) return undefined;
    const timer = setTimeout(() => {
      void navigate({ to: '/orgs/$orgSlug/overview', params: { orgSlug }, search: (previous) => ({ ...previous, q: text === '' ? undefined : text }), replace: true });
    }, 250);
    return () => {
      clearTimeout(timer);
    };
  }, [text, search.q, navigate, orgSlug]);

  const flows = useQuery(orgFlowsQueryOptions(orgId, { sort, activationState: search.state, q: search.q }));
  const toggle = useSetFlowActive(orgId);
  const byId = useMemo(() => new Map(projects.map((project) => [project.id, { name: project.name, handle: project.handle }])), [projects]);
  const filtered = search.q !== undefined || search.state !== undefined;

  const setState = (state: 'ACTIVE' | 'INACTIVE') => {
    void navigate({ to: '/orgs/$orgSlug/overview', params: { orgSlug }, search: (previous) => ({ ...previous, state: previous.state === state ? undefined : state }), replace: true });
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative w-full md:w-panel-width">
          <Search aria-hidden className="text-fg-subtle pointer-events-none absolute top-1/2 left-2 size-icon-16 -translate-y-1/2" />
          <Input
            type="search"
            aria-label={intl.formatMessage(messages.search)}
            placeholder={intl.formatMessage(messages.search)}
            value={text}
            onChange={(event) => {
              setText(event.target.value);
            }}
            className="pl-8"
          />
        </div>
        <FilterChip
          label={intl.formatMessage(messages.filterActive)}
          pressed={search.state === 'ACTIVE'}
          className="max-md:hidden"
          onClick={() => {
            setState('ACTIVE');
          }}
        />
        <FilterChip
          label={intl.formatMessage(messages.filterInactive)}
          pressed={search.state === 'INACTIVE'}
          className="max-md:hidden"
          onClick={() => {
            setState('INACTIVE');
          }}
        />
        <span className="flex-1 max-md:hidden" />
        <Select
          aria-label={intl.formatMessage(messages.sort)}
          value={sort}
          onValueChange={(value) => {
            void navigate({ to: '/orgs/$orgSlug/overview', params: { orgSlug }, search: (previous) => ({ ...previous, sort: value === '-lastExecutionAt' ? '-lastExecutionAt' : undefined }), replace: true });
          }}
          options={[
            { value: '-updatedAt', label: intl.formatMessage(messages.sortUpdated) },
            { value: '-lastExecutionAt', label: intl.formatMessage(messages.sortLastRun) },
          ]}
          className="w-panel-width max-md:hidden"
        />
      </div>
      <QueryState
        query={flows}
        skeletonRows={4}
        empty={
          filtered
            ? { title: intl.formatMessage(messages.noMatchTitle), description: intl.formatMessage(messages.noMatchText) }
            : {
                title: intl.formatMessage(messages.emptyTitle),
                description: intl.formatMessage(messages.emptyText),
                action: <Button onClick={onCreate}>{intl.formatMessage(messages.createFlow)}</Button>,
                illustration: <Mascot pose="thinking" />,
              }
        }
      >
        {(items) => (
          <FlowRows
            flows={items}
            orgSlug={orgSlug}
            projects={byId}
            busyFlowId={toggle.isPending ? toggle.variables.flow.id : undefined}
            failure={toggle.error ?? undefined}
            onToggle={(flow, active) => {
              toggle.mutate({ flow, active });
            }}
          />
        )}
      </QueryState>
    </div>
  );
}
