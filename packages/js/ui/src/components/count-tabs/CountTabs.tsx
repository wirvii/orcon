// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { MouseEvent } from 'react';

import { cn } from '../../lib/cn';

export interface CountTab {
  id: string;
  label: string;
  /** How many things are in that tab. Left out when it is not known: nothing is shown instead of a zero. */
  count?: number | undefined;
  href: string;
}

export interface CountTabsProps {
  tabs: readonly CountTab[];
  /** The `id` of the tab being shown. */
  currentId: string;
  /** Names the list of tabs for assistive technology. */
  label: string;
  /**
   * Called instead of following the link on a plain click, so a router can change the address without
   * reloading. A click with a modifier key still uses the link.
   */
  onNavigate?: ((tab: CountTab) => void) | undefined;
  className?: string | undefined;
}

/**
 * Tabs that are links: each one has an address, so the view is in the URL and can be opened in a new tab.
 * The current tab has a bar under it and a heavier weight as well as a stronger color. A count sits next to
 * the label in a small pill.
 */
export function CountTabs({ tabs, currentId, label, onNavigate, className }: CountTabsProps) {
  const follow = (tab: CountTab) => (event: MouseEvent<HTMLAnchorElement>) => {
    if (!onNavigate || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    onNavigate(tab);
  };
  return (
    <nav aria-label={label} className={cn('border-border border-b', className)}>
      <ul className="flex gap-1 overflow-x-auto" data-truncate>
        {tabs.map((tab) => {
          const current = tab.id === currentId;
          return (
            <li key={tab.id} className="shrink-0">
              <a
                href={tab.href}
                aria-current={current ? 'page' : undefined}
                onClick={follow(tab)}
                className={cn(
                  'text-fg-subtle relative inline-flex min-h-target h-10 items-center gap-1 rounded-md px-control-x text-base font-medium whitespace-nowrap transition-colors',
                  'hover:text-fg',
                  // The bar of the current tab is drawn inside the link, so the focus ring can follow its radius.
                  'after:absolute after:inset-x-2 after:bottom-0 after:border-b-2 after:border-transparent',
                  current && 'text-fg font-semibold after:border-canvas-selection',
                )}
              >
                {tab.label}
                {tab.count === undefined ? null : (
                  <span className="bg-status-neutral-bg text-status-neutral rounded-full px-1 text-2xs font-semibold">{tab.count}</span>
                )}
              </a>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
