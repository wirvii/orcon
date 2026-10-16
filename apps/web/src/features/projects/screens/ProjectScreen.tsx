// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Breadcrumb, Button, CountTabs, type CountTab } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Outlet, useNavigate, useRouter, useRouterState } from '@tanstack/react-router';
import { Plus } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { projectByHandleQueryOptions } from '../api/queries';

const messages = defineMessages({
  tabs: { id: 'project.tabs', defaultMessage: 'Sections of the project', description: 'Accessible name of the tabs of a project: flows, credentials, executions and settings.' },
  flows: { id: 'project.tab.flows', defaultMessage: 'Flows', description: 'Tab of a project that lists its flows.' },
  credentials: { id: 'project.tab.credentials', defaultMessage: 'Credentials', description: 'Tab of a project that lists its credentials.' },
  executions: { id: 'project.tab.executions', defaultMessage: 'Executions', description: 'Tab of a project that lists its executions.' },
  settings: { id: 'project.tab.settings', defaultMessage: 'Settings', description: 'Tab of a project with its settings.' },
  projects: { id: 'project.breadcrumb.projects', defaultMessage: 'Projects', description: 'Step of the breadcrumb of a project that stands for the list of projects.' },
  create: { id: 'project.createFlow', defaultMessage: 'Create flow', description: 'Button on the page of a project that starts a new flow in it.' },
});

const route = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle');

const tabIds = ['flows', 'credentials', 'executions', 'settings'] as const;
type TabId = (typeof tabIds)[number];

/** The tab an address belongs to, or nothing for a page inside the project (the editor, one execution). */
function tabOf(pathname: string, base: string): TabId | undefined {
  if (!pathname.startsWith(`${base}/`)) return undefined;
  const [first, second] = pathname.slice(base.length + 1).split('/').filter(Boolean);
  if (second !== undefined) return undefined;
  return tabIds.find((id) => id === first);
}

/**
 * The page of a project: its name, a way to create a flow, and the tabs Flows, Credentials, Executions and Settings.
 * The header and the tabs show only on those four pages; the editor and one execution fill the space themselves.
 * The people of the project are not shown: the contract has no list of project members yet (SPEC-019 F-04).
 */
export function ProjectScreen() {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate();
  const { organization, project: opened } = route.useRouteContext();
  const { orgSlug, projectHandle } = route.useParams();
  // The route read the project once on the way in; this keeps its flow count current as flows are created.
  const fresh = useQuery(projectByHandleQueryOptions(organization.id, opened.handle));
  const project = fresh.data?.[0] ?? opened;
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const base = router.buildLocation({ to: '/orgs/$orgSlug/projects/$projectHandle', params: { orgSlug, projectHandle } }).pathname.replace(/\/$/, '');
  const current = tabOf(pathname, base);
  if (current === undefined) return <Outlet />;

  const tabs: CountTab[] = [
    { id: 'flows', label: intl.formatMessage(messages.flows), count: project.flowCount, href: `${base}/flows` },
    { id: 'credentials', label: intl.formatMessage(messages.credentials), href: `${base}/credentials` },
    { id: 'executions', label: intl.formatMessage(messages.executions), href: `${base}/executions` },
    { id: 'settings', label: intl.formatMessage(messages.settings), href: `${base}/settings` },
  ];

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="flex min-w-0 flex-col gap-2">
          <Breadcrumb items={[{ label: organization.name }, { label: intl.formatMessage(messages.projects) }, { label: project.name }]} className="max-md:hidden" />
          <div className="flex min-w-0 items-center gap-3">
            <span aria-hidden className="bg-status-success text-fg-inverse inline-flex size-8 shrink-0 items-center justify-center rounded-md text-md font-bold">
              {(project.name.trim().charAt(0) || '?').toUpperCase()}
            </span>
            <h1 className="text-xl font-semibold wrap-anywhere">{project.name}</h1>
          </div>
        </div>
        <Button
          onClick={() => {
            void navigate({ to: '/orgs/$orgSlug/projects/$projectHandle/flows', params: { orgSlug, projectHandle }, search: { create: true } });
          }}
        >
          <Plus aria-hidden className="size-icon-16" />
          {intl.formatMessage(messages.create)}
        </Button>
      </header>
      <CountTabs
        tabs={tabs}
        currentId={current}
        label={intl.formatMessage(messages.tabs)}
        onNavigate={(tab) => {
          void navigate({ href: tab.href });
        }}
      />
      <Outlet />
    </div>
  );
}
