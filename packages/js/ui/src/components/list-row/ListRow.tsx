// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { AnchorHTMLAttributes, MouseEvent, ReactNode, Ref } from 'react';

import { cn } from '../../lib/cn';

export interface ListRowProps extends Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'title' | 'children'> {
  ref?: Ref<HTMLAnchorElement> | undefined;
  /** What the row is: the name that is read out for the whole row. */
  title: string;
  /** A second line under the title: the project, a tag, the state. */
  meta?: ReactNode | undefined;
  /** At the start of the row: an icon or a letter tile. */
  leading?: ReactNode | undefined;
  /** At the end of the row: a state, a time, a switch. It sits beside the row's link, so a switch or a button is valid there. */
  trailing?: ReactNode | undefined;
  /**
   * Called instead of following the link on a plain click, so a router can change the address without
   * reloading. A click with a modifier key still uses the link.
   */
  onNavigate?: (() => void) | undefined;
  /** Classes of the row itself, not of its link. */
  className?: string | undefined;
}

/**
 * A dense row of a list. The row is one link, stretched over its whole area, with a visible focus ring: a
 * click anywhere on it opens the item, and no text inside it is a link of its own. Long titles are cut with an
 * ellipsis (declared with `data-truncate`); the full name stays in the accessible name. What is in `trailing`
 * (a switch, a menu button) is a separate control beside the link, not inside it.
 */
export function ListRow({ title, meta, leading, trailing, onNavigate, onClick, className, ref, ...props }: ListRowProps) {
  const follow = (event: MouseEvent<HTMLAnchorElement>) => {
    onClick?.(event);
    if (!onNavigate || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    onNavigate();
  };
  return (
    <div
      className={cn(
        'border-border text-fg hover:bg-surface-hover focus-within:bg-surface-hover relative flex min-h-target items-center gap-3 border-b px-4 py-3 transition-colors last:border-b-0',
        className,
      )}
    >
      {leading ? <span className="shrink-0">{leading}</span> : null}
      <a
        ref={ref}
        onClick={follow}
        className="flex min-w-0 flex-1 flex-col gap-1 rounded-sm after:absolute after:inset-0"
        {...props}
      >
        <span data-truncate className="text-md truncate font-semibold">
          {title}
        </span>
        {meta ? (
          <span data-truncate className="text-fg-muted truncate text-sm">
            {meta}
          </span>
        ) : null}
      </a>
      {trailing ? <span className="text-fg-muted relative flex shrink-0 items-center gap-2 text-sm">{trailing}</span> : null}
    </div>
  );
}
