// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { LucideIcon } from 'lucide-react';
import { useState, type MouseEvent, type ReactNode } from 'react';
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
  badge?: number | undefined;
  /**
   * The link leaves the single-page app: the server answers it with its own page, so a plain click
   * follows the link and never goes through `onNavigate`.
   */
  fullPage?: boolean | undefined;
}

export interface AppShellProps {
  /**
   * The sections of the simple frame: a top bar, a sidebar or rail and a bottom bar. Used when `sidebar` is not
   * given (screen templates, stories).
   */
  navigation?: readonly NavEntry[] | undefined;
  /**
   * The sidebar of the product (a SidebarNav). When it is given the shell is the product frame: the sidebar on
   * the left from md up, the page filling the rest and scrolling inside it, and on a phone `phoneBar` on top and
   * `tabBar` along the bottom instead of the sidebar. `navigation`, `brand` and `actions` are not used then.
   */
  sidebar?: ReactNode | undefined;
  /** The bar across the top of a phone (a PhoneTopBar). */
  phoneBar?: ReactNode | undefined;
  /** The five destinations along the bottom of a phone (a BottomTabBar). */
  tabBar?: ReactNode | undefined;
  /** The `id` of the entry being shown. */
  currentId?: string | undefined;
  /** Shown at the start of the top bar, usually the BrandMark. */
  brand?: ReactNode | undefined;
  /** Controls at the end of the top bar. */
  actions?: ReactNode | undefined;
  children: ReactNode;
  /** Open the drawer when the page loads. Only for stories and tests. */
  defaultDrawerOpen?: boolean | undefined;
  /**
   * The shell is exactly one viewport tall and the page content fills the space
   * between the bars, scrolling inside it. For screens that need a definite
   * height, such as the flow editor.
   */
  fill?: boolean | undefined;
  /**
   * Called instead of following the link when a person activates an entry with a
   * plain click, so a router can navigate without reloading the page. Clicks with a
   * modifier key and middle clicks still use the link.
   */
  onNavigate?: ((entry: NavEntry) => void) | undefined;
}

/**
 * The frame of the app. From lg the sidebar lists sections with names; from md to
 * lg it is a rail of icons; below md the sections are a bottom bar plus a drawer
 * opened from the top bar. A skip link jumps to the content.
 */
export function AppShell({ sidebar, phoneBar, tabBar, ...props }: AppShellProps) {
  if (sidebar !== undefined) {
    return (
      <ProductFrame sidebar={sidebar} phoneBar={phoneBar} tabBar={tabBar} fill={props.fill ?? false}>
        {props.children}
      </ProductFrame>
    );
  }
  return <SimpleFrame {...props} />;
}

interface ProductFrameProps {
  sidebar: ReactNode;
  phoneBar?: ReactNode | undefined;
  tabBar?: ReactNode | undefined;
  fill: boolean;
  children: ReactNode;
}

/**
 * The frame of the product: the sidebar is a full-height column from md up; on a phone the top bar and the tab bar
 * take its place. The page scrolls inside the frame, which is exactly one viewport tall.
 */
function ProductFrame({ sidebar, phoneBar, tabBar, fill, children }: ProductFrameProps) {
  const intl = useIntl();
  return (
    <div className="bg-app flex h-screen">
      <a
        href="#main-content"
        className="text-fg-link sr-only focus:not-sr-only focus:bg-surface focus:absolute focus:top-2 focus:left-2 focus:z-toast focus:inline-flex focus:min-h-target focus:items-center focus:rounded-md focus:px-3"
      >
        {intl.formatMessage(messages.skip)}
      </a>
      <div className="hidden h-full md:flex">{sidebar}</div>
      <div className="flex min-w-0 flex-1 flex-col">
        {phoneBar ? <div className="md:hidden">{phoneBar}</div> : null}
        <main
          id="main-content"
          tabIndex={-1}
          className={cn('min-h-0 min-w-0 flex-1 overflow-y-auto outline-hidden', fill ? 'flex flex-col' : 'px-4 pt-4 pb-8 md:px-8 md:pt-6 md:pb-12')}
        >
          {fill ? children : <div className="mx-auto flex w-full max-w-screen-xl flex-col gap-6">{children}</div>}
        </main>
        {tabBar ? <div className="md:hidden">{tabBar}</div> : null}
      </div>
    </div>
  );
}

function SimpleFrame({ navigation = [], currentId, brand, actions, children, defaultDrawerOpen = false, fill = false, onNavigate }: Omit<AppShellProps, 'sidebar' | 'phoneBar' | 'tabBar'>) {
  const intl = useIntl();
  const [drawerOpen, setDrawerOpen] = useState(defaultDrawerOpen);
  const hasNavigation = navigation.length > 0;
  const follow = (entry: NavEntry, afterwards?: () => void) => (event: MouseEvent<HTMLAnchorElement>) => {
    afterwards?.();
    if (!onNavigate || entry.fullPage || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    onNavigate(entry);
  };

  return (
    <div className={cn('bg-app flex flex-col', fill ? 'h-screen' : 'min-h-screen')}>
      <a
        href="#main-content"
        className="text-fg-link sr-only focus:not-sr-only focus:bg-surface focus:absolute focus:top-2 focus:left-2 focus:z-toast focus:inline-flex focus:min-h-target focus:items-center focus:rounded-md focus:px-3"
      >
        {intl.formatMessage(messages.skip)}
      </a>
      <TopBar brand={brand} actions={actions} onOpenNavigation={hasNavigation ? () => setDrawerOpen(true) : undefined} />
      <div className="flex min-h-0 flex-1">
        {hasNavigation ? (
          <nav
            aria-label={intl.formatMessage(messages.main)}
            className="bg-surface border-border hidden shrink-0 flex-col gap-1 border-r p-2 md:flex md:w-16 lg:w-panel-width"
          >
            {navigation.map((entry) => (
              <NavItem key={entry.id} href={entry.href} icon={entry.icon} label={entry.label} current={entry.id === currentId} badge={entry.badge} responsiveLabel onClick={follow(entry)} />
            ))}
          </nav>
        ) : null}
        <main
          id="main-content"
          tabIndex={-1}
          className={cn('min-w-0 flex-1 p-4 outline-hidden', hasNavigation && 'pb-16 md:pb-4', fill && 'flex min-h-0 flex-col overflow-y-auto')}
        >
          {children}
        </main>
      </div>
      {hasNavigation ? (
        <>
          <nav
            aria-label={intl.formatMessage(messages.main)}
            className="bg-surface border-border z-sticky fixed inset-x-0 bottom-0 flex justify-around border-t md:hidden"
          >
            {navigation.slice(0, 5).map((entry) => (
              <NavItem key={entry.id} href={entry.href} icon={entry.icon} label={entry.label} current={entry.id === currentId} collapsed className="rounded-none" onClick={follow(entry)} />
            ))}
          </nav>
          <Sheet open={drawerOpen} onOpenChange={setDrawerOpen} title={intl.formatMessage(messages.drawerTitle)} side="bottom">
            <nav aria-label={intl.formatMessage(messages.main)} className="flex flex-col gap-1">
              {navigation.map((entry) => (
                <NavItem key={entry.id} href={entry.href} icon={entry.icon} label={entry.label} current={entry.id === currentId} badge={entry.badge} onClick={follow(entry, () => setDrawerOpen(false))} />
              ))}
            </nav>
          </Sheet>
        </>
      ) : null}
    </div>
  );
}
