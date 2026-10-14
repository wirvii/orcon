// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { AppShell, BrandMark, type NavEntry } from '@orcon/ui';
import { linkOptions, useNavigate, useRouter, useRouterState } from '@tanstack/react-router';
import { Braces, Settings } from 'lucide-react';
import type { ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { ConnectionBanners } from './ConnectionBanners';

const messages = defineMessages({
  settings: { id: 'app.nav.settings', defaultMessage: 'Settings', description: 'Navigation entry for the settings pages.' },
  api: {
    id: 'app.nav.api',
    defaultMessage: 'API reference',
    description: 'Navigation entry that opens the API reference page, which the server serves outside the app.',
  },
});

const settingsLink = linkOptions({ to: '/settings/appearance' });

export interface PageShellProps {
  /** The page fills the space between the bars and scrolls inside it (the flow editor). */
  fill?: boolean | undefined;
  /** Controls at the end of the top bar, such as the user menu of a signed-in person. */
  actions?: ReactNode | undefined;
  children: ReactNode;
}

/**
 * The frame of every page: the navigation the app always offers, the connection banners and the
 * page. Pages that need a session add their own entries and controls through `actions`.
 */
export function PageShell({ fill = false, actions, children }: PageShellProps) {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate();
  const pathname = useRouterState({ select: (state) => state.location.pathname });

  const navigation: NavEntry[] = [
    { id: 'settings', icon: Settings, href: router.buildLocation(settingsLink).href, label: intl.formatMessage(messages.settings) },
    // The API reference is a page the server answers, not a route of the app: a plain link that loads it.
    { id: 'api', icon: Braces, href: '/api/reference/', label: intl.formatMessage(messages.api), fullPage: true },
  ];

  return (
    <AppShell
      fill={fill}
      navigation={navigation}
      currentId={pathname.startsWith('/settings') ? 'settings' : undefined}
      brand={<BrandMark />}
      actions={actions}
      onNavigate={(entry) => {
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
