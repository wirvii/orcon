// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { LucideIcon } from 'lucide-react';
import type { MouseEvent } from 'react';

import { cn } from '../../lib/cn';

export interface BottomTab {
  id: string;
  /** The name of the destination. Always visible: an icon alone is not a label. */
  label: string;
  href: string;
  icon: LucideIcon;
}

export interface BottomTabBarProps {
  /** The destinations, left to right. A phone has room for five. */
  tabs: readonly BottomTab[];
  /** The `id` of the destination being shown. */
  currentId?: string | undefined;
  /** Names the navigation landmark. */
  label: string;
  /**
   * Called instead of following the link on a plain click, so a router can change the address without
   * reloading. A click with a modifier key still uses the link.
   */
  onNavigate?: ((tab: BottomTab) => void) | undefined;
  className?: string | undefined;
}

/**
 * The navigation of a phone: up to five destinations along the bottom, each with its name under the icon.
 * The current one is heavier, brighter and has an accent icon; the page is also marked with `aria-current`.
 */
export function BottomTabBar({ tabs, currentId, label, onNavigate, className }: BottomTabBarProps) {
  const follow = (tab: BottomTab) => (event: MouseEvent<HTMLAnchorElement>) => {
    if (!onNavigate || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    onNavigate(tab);
  };
  return (
    <nav aria-label={label} className={cn('bg-surface border-border border-t', className)}>
      <ul className="grid grid-cols-5">
        {tabs.slice(0, 5).map((tab) => {
          const current = tab.id === currentId;
          const Icon = tab.icon;
          return (
            <li key={tab.id} className="min-w-target">
              <a
                href={tab.href}
                aria-current={current ? 'page' : undefined}
                onClick={follow(tab)}
                className={cn(
                  'text-fg-subtle flex min-h-16 flex-col items-center justify-center gap-1 px-1 text-2xs font-medium transition-colors',
                  'hover:text-fg',
                  current && 'text-fg font-semibold',
                )}
              >
                <Icon aria-hidden className={cn('size-icon-20 shrink-0', current && 'text-fg-accent')} />
                <span data-truncate className="max-w-full truncate">
                  {tab.label}
                </span>
              </a>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
