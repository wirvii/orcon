// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Inbox, ListChecks, Settings, Workflow } from 'lucide-react';
import type { ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { AppShell, type NavEntry } from '../components/app-shell/AppShell';
import { BrandMark } from '../components/brand-mark/BrandMark';
import { NotificationBell } from '../components/notification-bell/NotificationBell';

const messages = defineMessages({
  flows: { id: 'screen.nav.flows', defaultMessage: 'Flows', description: 'Navigation entry for the list of flows.' },
  executions: { id: 'screen.nav.executions', defaultMessage: 'Executions', description: 'Navigation entry for the list of executions (runs of a flow).' },
  attention: { id: 'screen.nav.attention', defaultMessage: 'Attention', description: 'Navigation entry for the inbox of things that need a person.' },
  settings: { id: 'screen.nav.settings', defaultMessage: 'Settings', description: 'Navigation entry for the settings pages.' },
});

export type ShellSection = 'flows' | 'executions' | 'attention' | 'settings';

export interface ProductShellProps {
  /** The section being shown. */
  current?: ShellSection | undefined;
  /** Items waiting for a person. Shown on the bell and on the Attention entry. */
  attentionCount?: number | undefined;
  /** The page content. Give it its own h1. */
  children: ReactNode;
  /** The shell fills the viewport and the content scrolls inside it. Used by the editor. */
  fill?: boolean | undefined;
}

/**
 * The frame every product screen shares: sidebar or rail or bottom bar by
 * breakpoint, the brand, the attention bell, and a skip link.
 */
export function ProductShell({ current, attentionCount = 3, children, fill = false }: ProductShellProps) {
  const intl = useIntl();
  const navigation: NavEntry[] = [
    { id: 'flows', href: '#flows', label: intl.formatMessage(messages.flows), icon: Workflow },
    { id: 'executions', href: '#executions', label: intl.formatMessage(messages.executions), icon: ListChecks },
    { id: 'attention', href: '#attention', label: intl.formatMessage(messages.attention), icon: Inbox, badge: attentionCount > 0 ? attentionCount : undefined },
    { id: 'settings', href: '#settings', label: intl.formatMessage(messages.settings), icon: Settings },
  ];
  return (
    <AppShell navigation={navigation} currentId={current} brand={<BrandMark />} actions={<NotificationBell count={attentionCount} />} fill={fill}>
      {children}
    </AppShell>
  );
}
