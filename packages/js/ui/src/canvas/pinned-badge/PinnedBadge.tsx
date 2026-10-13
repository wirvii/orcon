// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Pin } from 'lucide-react';

import { cn } from '../../lib/cn';

export interface PinnedBadgeProps {
  /** Place the badge on the top-left corner of its parent (offset -8, -8). Off for a standalone badge. */
  corner?: boolean;
  className?: string;
}

/**
 * The pin on a node whose output is pinned: a 22 px circle on the top-left corner. Hidden from
 * assistive technology; the node's accessible name says "pinned" and the "Pinned" chip repeats it.
 */
export function PinnedBadge({ corner = true, className }: PinnedBadgeProps) {
  return (
    <span
      aria-hidden
      data-pinned-badge
      className={cn(
        'bg-surface-raised border-canvas text-fg-link z-10 grid size-canvas-badge shrink-0 place-items-center rounded-full border-2 shadow-sm',
        corner && 'absolute -top-2 -left-2',
        className,
      )}
    >
      <Pin className="size-3" />
    </span>
  );
}
