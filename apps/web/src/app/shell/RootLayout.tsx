// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { AppShell, Banner, BrandMark, type NavEntry } from '@orcon/ui';
import { onlineManager } from '@tanstack/react-query';
import { createRootRouteWithContext, linkOptions, Outlet, useNavigate, useRouter, useRouterState } from '@tanstack/react-router';
import { House, Settings, Workflow } from 'lucide-react';
import { useSyncExternalStore } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { RouteError } from '../errors/RouteError';
import { useLiveEventsState } from '../live-events/LiveEventsProvider';
import type { RouterContext } from '../router-context';
import { NotFound } from './NotFound';

const messages = defineMessages({
  home: { id: 'app.nav.home', defaultMessage: 'Start', description: 'Navigation entry that goes to the start page.' },
  editor: {
    id: 'app.nav.editor',
    defaultMessage: 'Flow editor',
    description: 'Navigation entry that opens the flow editor on the sample flow, until the list of flows exists.',
  },
  settings: { id: 'app.nav.settings', defaultMessage: 'Settings', description: 'Navigation entry for the settings pages.' },
  offlineTitle: {
    id: 'app.offline.title',
    defaultMessage: 'You are offline',
    description: 'Title of the banner shown while the browser has no network connection.',
  },
  offlineText: {
    id: 'app.offline.text',
    defaultMessage: 'What you see may be out of date. Requests resume when the connection returns.',
    description: 'Explains the offline banner: data may be stale and requests wait for the network.',
  },
  pausedTitle: {
    id: 'app.live.paused.title',
    defaultMessage: 'Live updates are paused',
    description: 'Title of the banner shown while the connection that delivers live changes is down.',
  },
  pausedText: {
    id: 'app.live.paused.text',
    defaultMessage: 'The page reconnects by itself and refreshes what it shows.',
    description: 'Explains the banner about paused live updates: the page reconnects and refreshes.',
  },
});

/**
 * The sample flow the editor opens until flows are stored on the server (the same id as the editor's
 * sample, which `app/` may not import): the navigation entry goes to a stable address. The list of
 * flows replaces this entry.
 */
const SAMPLE_FLOW_ID = '01930000-0000-7000-8000-00000000ed5a';

// Typed destinations of the main navigation: a renamed route fails the build here.
const destinations = [
  { id: 'home', icon: House, link: linkOptions({ to: '/' }) },
  { id: 'editor', icon: Workflow, link: linkOptions({ to: '/flows/$flowId', params: { flowId: SAMPLE_FLOW_ID } }) },
  { id: 'settings', icon: Settings, link: linkOptions({ to: '/settings/appearance' }) },
] as const;

const subscribeOnline = (onChange: () => void) => onlineManager.subscribe(onChange);
const isOnline = () => onlineManager.isOnline();

function RootLayout() {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const liveState = useLiveEventsState();
  const online = useSyncExternalStore(subscribeOnline, isOnline);

  const navigation: NavEntry[] = destinations.map(({ id, icon, link }) => ({
    id,
    icon,
    href: router.buildLocation(link).href,
    label: intl.formatMessage(messages[id]),
  }));
  const current = destinations.find(({ link }) => (link.to === '/' ? pathname === '/' : link.to === '/flows/$flowId' ? pathname.startsWith('/flows/') : pathname.startsWith('/settings')))?.id;
  // The editor needs a page of a definite height to fill; every other page grows with its content.
  const editing = pathname.startsWith('/flows/');

  return (
    <AppShell
      fill={editing}
      navigation={navigation}
      currentId={current}
      brand={<BrandMark />}
      onNavigate={(entry) => {
        const target = destinations.find(({ id }) => id === entry.id);
        if (target) void navigate(target.link);
      }}
    >
      <div className={editing ? 'flex min-h-0 flex-1 flex-col gap-3' : 'flex flex-col gap-3'}>
        {online ? null : (
          <Banner tone="warning" title={intl.formatMessage(messages.offlineTitle)}>
            {intl.formatMessage(messages.offlineText)}
          </Banner>
        )}
        {liveState === 'paused' ? (
          <Banner tone="warning" title={intl.formatMessage(messages.pausedTitle)}>
            {intl.formatMessage(messages.pausedText)}
          </Banner>
        ) : null}
        <Outlet />
      </div>
    </AppShell>
  );
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  errorComponent: RouteError,
  notFoundComponent: NotFound,
});
