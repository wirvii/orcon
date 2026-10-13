// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { LucideIcon } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { NavItem } from '../nav-item/NavItem';
import { Sheet } from '../sheet/Sheet';
import { TopBar } from '../top-bar/TopBar';

const messages = defineMessages({
  main: {
    id: 'nav.main',
    defaultMessage: 'Main navigation',
    description: 'Accessible name of the navigation landmark with the main sections of the app.',
  },
  skip: {
    id: 'nav.skipToContent',
    defaultMessage: 'Skip to main content',
    description: 'Link, visible when focused, that jumps over the navigation to the page content.',
  },
  drawerTitle: {
    id: 'nav.drawerTitle',
    defaultMessage: 'Navigation',
    description: 'Title of the drawer that lists the main sections of the app on small screens.',
  },
});

export interface NavEntry {
  id: string;
  href: string;
  label: string;
  icon: LucideIcon;
  /** Count of things waiting in that section. */
  badge?: number;
}

export interface AppShellProps {
  navigation: readonly NavEntry[];
  /** The `id` of the entry being shown. */
  currentId?: string;
  /** Shown at the start of the top bar, usually the BrandMark. */
  brand?: ReactNode;
  /** Controls at the end of the top bar. */
  actions?: ReactNode;
  children: ReactNode;
  /** Open the drawer when the page loads. Only for stories and tests. */
  defaultDrawerOpen?: boolean;
  /**
   * The shell is exactly one viewport tall and the page content fills the space
   * between the bars, scrolling inside it. For screens that need a definite
   * height, such as the flow editor.
   */
  fill?: boolean;
}

/**
 * The frame of the app. From lg the sidebar lists sections with names; from md to
 * lg it is a rail of icons; below md the sections are a bottom bar plus a drawer
 * opened from the top bar. A skip link jumps to the content.
 */
export function AppShell({ navigation, currentId, brand, actions, children, defaultDrawerOpen = false, fill = false }: AppShellProps) {
  const intl = useIntl();
  const [drawerOpen, setDrawerOpen] = useState(defaultDrawerOpen);

  return (
    <div className={cn('bg-app flex flex-col', fill ? 'h-screen' : 'min-h-screen')}>
      <a
        href="#main-content"
        className="text-fg-link sr-only focus:not-sr-only focus:bg-surface focus:absolute focus:top-2 focus:left-2 focus:z-toast focus:inline-flex focus:min-h-target focus:items-center focus:rounded-md focus:px-3"
      >
        {intl.formatMessage(messages.skip)}
      </a>
      <TopBar brand={brand} actions={actions} onOpenNavigation={() => setDrawerOpen(true)} />
      <div className="flex min-h-0 flex-1">
        <nav
          aria-label={intl.formatMessage(messages.main)}
          className="bg-surface border-border hidden shrink-0 flex-col gap-1 border-r p-2 md:flex md:w-16 lg:w-panel-width"
        >
          {navigation.map((entry) => (
            <NavItem key={entry.id} href={entry.href} icon={entry.icon} label={entry.label} current={entry.id === currentId} badge={entry.badge} responsiveLabel />
          ))}
        </nav>
        <main
          id="main-content"
          tabIndex={-1}
          className={cn('min-w-0 flex-1 p-4 pb-16 outline-hidden md:pb-4', fill && 'flex min-h-0 flex-col overflow-y-auto')}
        >
          {children}
        </main>
      </div>
      <nav
        aria-label={intl.formatMessage(messages.main)}
        className="bg-surface border-border z-sticky fixed inset-x-0 bottom-0 flex justify-around border-t md:hidden"
      >
        {navigation.slice(0, 5).map((entry) => (
          <NavItem key={entry.id} href={entry.href} icon={entry.icon} label={entry.label} current={entry.id === currentId} collapsed className="rounded-none" />
        ))}
      </nav>
      <Sheet open={drawerOpen} onOpenChange={setDrawerOpen} title={intl.formatMessage(messages.drawerTitle)} side="bottom">
        <nav aria-label={intl.formatMessage(messages.main)} className="flex flex-col gap-1">
          {navigation.map((entry) => (
            <NavItem key={entry.id} href={entry.href} icon={entry.icon} label={entry.label} current={entry.id === currentId} badge={entry.badge} onClick={() => setDrawerOpen(false)} />
          ))}
        </nav>
      </Sheet>
    </div>
  );
}
