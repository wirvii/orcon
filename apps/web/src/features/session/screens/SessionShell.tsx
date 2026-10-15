// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { AppShell, BrandMark, type NavEntry } from '@orcon/ui';
import { linkOptions, useNavigate, useParams, useRouter, useRouterState } from '@tanstack/react-router';
import { Braces, FolderKanban, Settings } from 'lucide-react';
import type { ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { ConnectionBanners } from '../../../app/shell/ConnectionBanners';
import { UserMenu } from './UserMenu';

const messages = defineMessages({
  projects: {
    id: 'session.nav.projects',
    defaultMessage: 'Projects',
    description: 'Navigation entry that opens the projects of the organization being used.',
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
  const { orgSlug } = useParams({ strict: false });
  const projectsLink = orgSlug === undefined ? undefined : linkOptions({ to: '/orgs/$orgSlug/projects', params: { orgSlug } });

  const navigation: NavEntry[] = [
    ...(projectsLink === undefined
      ? []
      : [{ id: 'projects', icon: FolderKanban, href: router.buildLocation(projectsLink).href, label: intl.formatMessage(messages.projects) }]),
    { id: 'settings', icon: Settings, href: router.buildLocation(settingsLink).href, label: intl.formatMessage(messages.settings) },
    // The API reference is a page the server answers, not a route of the app: a plain link that loads it.
    { id: 'api', icon: Braces, href: '/api/reference/', label: intl.formatMessage(messages.api), fullPage: true },
  ];

  const currentId = pathname.includes('/projects') ? 'projects' : pathname.startsWith('/settings') ? 'settings' : undefined;

  return (
    <AppShell
      navigation={navigation}
      currentId={currentId}
      brand={<BrandMark />}
      actions={<UserMenu />}
      onNavigate={(entry) => {
        if (entry.id === 'projects' && projectsLink !== undefined) void navigate(projectsLink);
        if (entry.id === 'settings') void navigate(settingsLink);
      }}
    >
      <div className="flex flex-col gap-3">
        <ConnectionBanners />
        {children}
      </div>
    </AppShell>
  );
}
