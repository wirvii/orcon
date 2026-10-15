// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { AppShell, BrandMark, type NavEntry } from '@orcon/ui';
import { linkOptions, useMatches, useNavigate, useParams, useRouter, useRouterState } from '@tanstack/react-router';
import { Braces, FolderKanban, KeyRound, Settings } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { useEffect, type ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { ConnectionBanners } from '../../../app/shell/ConnectionBanners';
import { organizationsQueryOptions } from '../api/queries';
import { rememberedOrganization, rememberOrganization } from '../api/last-org';
import { UserMenu } from './UserMenu';

const messages = defineMessages({
  projects: {
    id: 'session.nav.projects',
    defaultMessage: 'Projects',
    description: 'Navigation entry that opens the projects of the organization being used.',
  },
  credentials: {
    id: 'session.nav.credentials',
    defaultMessage: 'Credentials',
    description: 'Navigation entry that opens the credentials of the project being used.',
  },
  settings: { id: 'app.nav.settings', defaultMessage: 'Settings', description: 'Navigation entry for the settings pages.' },
  api: {
    id: 'app.nav.api',
    defaultMessage: 'API reference',
    description: 'Navigation entry that opens the API reference page, which the server serves outside the app.',
  },
});

const settingsLink = linkOptions({ to: '/settings/appearance' });

/**
 * The frame of the signed-in pages: the navigation (projects only while an organization is open), the
 * connection banners and the menu of the person who is signed in.
 */
export function SessionShell({ children }: { children: ReactNode }) {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const params = useParams({ strict: false });
  const { projectHandle } = params;
  // Outside an organization's pages (settings) the navigation still offers the projects of the organization the
  // person was in, or of the only one they belong to.
  const organizations = useQuery({ ...organizationsQueryOptions(), enabled: params.orgSlug === undefined });
  const onlyOrganization = organizations.data?.length === 1 ? organizations.data[0]?.slug : undefined;
  const orgSlug = params.orgSlug ?? rememberedOrganization() ?? onlyOrganization;
  useEffect(() => {
    if (params.orgSlug !== undefined) rememberOrganization(params.orgSlug);
  }, [params.orgSlug]);
  const projectsLink = orgSlug === undefined ? undefined : linkOptions({ to: '/orgs/$orgSlug/projects', params: { orgSlug } });
  const credentialsLink =
    orgSlug === undefined || projectHandle === undefined ? undefined : linkOptions({ to: '/orgs/$orgSlug/projects/$projectHandle/credentials', params: { orgSlug, projectHandle } });

  const navigation: NavEntry[] = [
    ...(projectsLink === undefined
      ? []
      : [{ id: 'projects', icon: FolderKanban, href: router.buildLocation(projectsLink).href, label: intl.formatMessage(messages.projects) }]),
    ...(credentialsLink === undefined
      ? []
      : [{ id: 'credentials', icon: KeyRound, href: router.buildLocation(credentialsLink).href, label: intl.formatMessage(messages.credentials) }]),
    { id: 'settings', icon: Settings, href: router.buildLocation(settingsLink).href, label: intl.formatMessage(messages.settings) },
    // The API reference is a page the server answers, not a route of the app: a plain link that loads it.
    { id: 'api', icon: Braces, href: '/api/reference/', label: intl.formatMessage(messages.api), fullPage: true },
  ];

  const currentId = pathname.endsWith('/credentials')
    ? 'credentials'
    : pathname.includes('/projects')
      ? 'projects'
      : pathname.startsWith('/settings')
        ? 'settings'
        : undefined;
  // A page that fills the space between the bars (the flow editor) declares it in its route's staticData.
  const fill = useMatches({ select: (matches) => matches.some((match) => (match.staticData as { fill?: boolean }).fill === true) });

  return (
    <AppShell
      fill={fill}
      navigation={navigation}
      currentId={currentId}
      brand={<BrandMark />}
      actions={<UserMenu />}
      onNavigate={(entry) => {
        if (entry.id === 'projects' && projectsLink !== undefined) void navigate(projectsLink);
        if (entry.id === 'credentials' && credentialsLink !== undefined) void navigate(credentialsLink);
        if (entry.id === 'settings') void navigate(settingsLink);
      }}
    >
      <div className={fill ? 'flex min-h-0 flex-1 flex-col gap-3' : 'flex flex-col gap-3'}>
        <ConnectionBanners />
        {children}
      </div>
    </AppShell>
  );
}
