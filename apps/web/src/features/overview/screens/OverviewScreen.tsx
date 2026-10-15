// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, EmptyState, ListRow, Mascot, PageHeader, ProblemAlert, RelativeTime, Skeleton } from '@orcon/ui';
import { useQueries, useQuery } from '@tanstack/react-query';
import { getRouteApi, useNavigate, useRouter } from '@tanstack/react-router';
import { CircleCheck, CircleDashed, Plus } from 'lucide-react';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { toProblemDetails } from '../../../app/errors/to-problem-details';
import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { QueryState } from '../../../app/query-state/QueryState';
import { shellFlowsQueryOptions, shellProjectsQueryOptions, type ShellFlow, type ShellProject } from '../../../app/shell/shell-data';
import { currentUserQueryOptions } from '../../../app/session/current-user';
import { identityOf } from '../../../app/session/identity';
import { greetingFor } from '../model/greeting';

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
  flows: { id: 'overview.flows.heading', defaultMessage: 'Flows', description: 'Heading of the list of flows on the home page.' },
  emptyTitle: { id: 'overview.empty.title', defaultMessage: 'No flows yet', description: 'Title of the home page list when no project has a flow.' },
  emptyText: {
    id: 'overview.empty.text',
    defaultMessage: 'Create your first flow. It starts inactive: nothing runs until you publish it.',
    description: 'Explains the empty list of flows on the home page and the first step.',
  },
  noProjectsTitle: { id: 'overview.noProjects.title', defaultMessage: 'No projects yet', description: 'Title of the home page when the organization has no project the person can read.' },
  noProjectsText: {
    id: 'overview.noProjects.text',
    defaultMessage: 'A project groups the flows of one area of work. Create one to start.',
    description: 'Explains the empty home page when there is no project and what to do.',
  },
  createProject: { id: 'overview.createProject', defaultMessage: 'Create a project', description: 'Button on the home page that opens the new project dialog.' },
  loading: { id: 'overview.loading', defaultMessage: 'Loading flows', description: 'Accessible name of the placeholders shown while the flows of the home page load.' },
  active: { id: 'overview.state.active', defaultMessage: 'Active', description: 'State of a flow that runs when its triggers fire, shown as text.' },
  inactive: { id: 'overview.state.inactive', defaultMessage: 'Inactive', description: 'State of a flow that does not run, shown as text.' },
});

const route = getRouteApi('/_session/orgs/$orgSlug');

interface FlowEntry {
  flow: ShellFlow;
  project: ShellProject;
}

/**
 * Home of an organization: a greeting and the flows of every project the person can read, most recently changed first.
 * The figures, the tabs and the Active switch of the approved design arrive with the endpoints they need
 * (SPEC-019 UX3); until then this lists what the API already gives, with nothing invented.
 */
export function OverviewScreen() {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate();
  const { organization } = route.useRouteContext();
  const { orgSlug } = route.useParams();
  const user = useQuery(currentUserQueryOptions());
  const projects = useQuery(shellProjectsQueryOptions(organization.id));

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/overview');
  }, []);

  const firstName = user.data === undefined ? '' : (identityOf(user.data).name.split(/\s+/)[0] ?? '');
  const greeting = { morning: messages.morning, afternoon: messages.afternoon, evening: messages.evening }[greetingFor(new Date().getHours())];

  const createFlow = () => {
    const first = projects.data?.[0];
    if (first === undefined) {
      void navigate({ to: '/orgs/$orgSlug/projects', params: { orgSlug }, search: { create: true } });
    } else {
      void navigate({ to: '/orgs/$orgSlug/projects/$projectHandle/flows', params: { orgSlug, projectHandle: first.handle }, search: { create: true } });
    }
  };

  const createButton = (
    <Button onClick={createFlow}>
      <Plus aria-hidden className="size-icon-16" />
      {intl.formatMessage(messages.createFlow)}
    </Button>
  );

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        breadcrumb={[{ label: organization.name }, { label: intl.formatMessage(messages.home) }]}
        title={intl.formatMessage(greeting, { name: firstName })}
        description={intl.formatMessage(messages.description, { organization: organization.name })}
        actions={createButton}
      />
      <QueryState
        query={projects}
        empty={{
          title: intl.formatMessage(messages.noProjectsTitle),
          description: intl.formatMessage(messages.noProjectsText),
          action: <Button onClick={createFlow}>{intl.formatMessage(messages.createProject)}</Button>,
          illustration: <Mascot pose="thinking" />,
        }}
      >
        {(items) => (
          <section aria-labelledby="overview-flows" className="flex flex-col gap-2">
            <h2 id="overview-flows" className="text-md font-semibold">
              {intl.formatMessage(messages.flows)}
            </h2>
            <FlowRows
              orgId={organization.id}
              projects={items}
              hrefOf={(handle, flowId) => router.buildLocation({ to: '/orgs/$orgSlug/projects/$projectHandle/flows/$flowId', params: { orgSlug, projectHandle: handle, flowId } }).href}
              onOpen={(handle, flowId) => void navigate({ to: '/orgs/$orgSlug/projects/$projectHandle/flows/$flowId', params: { orgSlug, projectHandle: handle, flowId } })}
              onCreate={createFlow}
            />
          </section>
        )}
      </QueryState>
    </div>
  );
}

interface FlowRowsProps {
  orgId: string;
  projects: readonly ShellProject[];
  hrefOf: (handle: string, flowId: string) => string;
  onOpen: (handle: string, flowId: string) => void;
  onCreate: () => void;
}

function FlowRows({ orgId, projects, hrefOf, onOpen, onCreate }: FlowRowsProps) {
  const intl = useIntl();
  const result = useQueries({
    queries: projects.map((project) => shellFlowsQueryOptions(orgId, project.id)),
    combine: (queries) => ({
      pending: queries.some((query) => query.status === 'pending'),
      error: queries.find((query) => query.status === 'error')?.error,
      data: queries.map((query) => query.data ?? []),
    }),
  });

  if (result.pending) {
    return (
      <div role="status" aria-busy="true" aria-label={intl.formatMessage(messages.loading)} className="flex flex-col gap-2">
        {[0, 1, 2].map((row) => (
          <Skeleton key={row} className="h-12 w-full" />
        ))}
      </div>
    );
  }
  if (result.error !== undefined) return <ProblemAlert problem={toProblemDetails(result.error)} />;

  const entries: FlowEntry[] = result.data
    .flatMap((flows, index) => flows.map((flow) => ({ flow, project: projects[index] })))
    .filter((entry): entry is FlowEntry => entry.project !== undefined)
    .sort((a, b) => b.flow.updatedAt.localeCompare(a.flow.updatedAt));

  if (entries.length === 0) {
    return (
      <EmptyState
        title={intl.formatMessage(messages.emptyTitle)}
        description={intl.formatMessage(messages.emptyText)}
        action={<Button onClick={onCreate}>{intl.formatMessage(messages.createFlow)}</Button>}
        mascot={<Mascot pose="thinking" />}
      />
    );
  }

  return (
    <div className="border-border bg-surface overflow-hidden rounded-lg border">
      {entries.map(({ flow, project }) => (
        <ListRow
          key={flow.id}
          href={hrefOf(project.handle, flow.id)}
          onNavigate={() => {
            onOpen(project.handle, flow.id);
          }}
          title={flow.name}
          meta={project.name}
          trailing={
            <>
              <RelativeTime value={flow.updatedAt} />
              {flow.activationState === 'ACTIVE' ? (
                <Badge tone="success" icon={<CircleCheck aria-hidden className="size-icon-16" />}>
                  {intl.formatMessage(messages.active)}
                </Badge>
              ) : (
                <Badge tone="neutral" icon={<CircleDashed aria-hidden className="size-icon-16" />}>
                  {intl.formatMessage(messages.inactive)}
                </Badge>
              )}
            </>
          }
        />
      ))}
    </div>
  );
}
