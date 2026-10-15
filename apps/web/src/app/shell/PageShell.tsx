// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { AppShell, BrandMark, type NavEntry } from '@orcon/ui';
import { linkOptions, useNavigate, useRouter, useRouterState } from '@tanstack/react-router';
import { Braces, Settings, Workflow } from 'lucide-react';
import type { ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { ConnectionBanners } from './ConnectionBanners';

const messages = defineMessages({
  editor: {
    id: 'app.nav.editor',
    defaultMessage: 'Flow editor',
    description: 'Navigation entry that opens the flow editor on the sample flow, until the list of flows exists.',
  },
  settings: { id: 'app.nav.settings', defaultMessage: 'Settings', description: 'Navigation entry for the settings pages.' },
  api: {
    id: 'app.nav.api',
    defaultMessage: 'API reference',
    description: 'Navigation entry that opens the API reference page, which the server serves outside the app.',
  },
});

/**
 * The sample flow the editor opens until flows are stored on the server (the same id as the editor's
 * sample, which `app/` may not import). The list of flows replaces this entry.
 */
const SAMPLE_FLOW_ID = '01930000-0000-7000-8000-00000000ed5a';

const editorLink = linkOptions({ to: '/flows/$flowId', params: { flowId: SAMPLE_FLOW_ID } });
const settingsLink = linkOptions({ to: '/settings/appearance' });

/** The navigation entry that owns the address, so the frame can mark it as the current page. */
function currentEntry(pathname: string): 'editor' | 'settings' | undefined {
  if (pathname.startsWith('/settings')) return 'settings';
  if (pathname.startsWith('/flows')) return 'editor';
  return undefined;
}

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
    { id: 'editor', icon: Workflow, href: router.buildLocation(editorLink).href, label: intl.formatMessage(messages.editor) },
    { id: 'settings', icon: Settings, href: router.buildLocation(settingsLink).href, label: intl.formatMessage(messages.settings) },
    // The API reference is a page the server answers, not a route of the app: a plain link that loads it.
    { id: 'api', icon: Braces, href: '/api/reference/', label: intl.formatMessage(messages.api), fullPage: true },
  ];

  return (
    <AppShell
      fill={fill}
      navigation={navigation}
      currentId={currentEntry(pathname)}
      brand={<BrandMark />}
      actions={actions}
      onNavigate={(entry) => {
        if (entry.id === 'editor') void navigate(editorLink);
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
