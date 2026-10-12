// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ScrollArea as ScrollAreaPrimitive } from 'radix-ui';
import type { ReactNode } from 'react';

import { cn } from '../../lib/cn';

const orientations = ['vertical', 'horizontal'] as const;

export interface ScrollAreaProps {
  /** Names the scrollable region. */
  label: string;
  children: ReactNode;
  /** Give the area a height (or max height) so it can overflow, for example `max-h-node-width`. */
  className?: string;
}

/**
 * A region with its own scrollbar. The region is focusable and labeled, so a
 * keyboard user can scroll it with the arrow keys.
 */
export function ScrollArea({ label, children, className }: ScrollAreaProps) {
  return (
    <ScrollAreaPrimitive.Root type="auto" className={cn('relative overflow-hidden', className)}>
      <ScrollAreaPrimitive.Viewport role="region" aria-label={label} tabIndex={0} className="size-full rounded-md">
        {children}
      </ScrollAreaPrimitive.Viewport>
      {orientations.map((orientation) => (
        <ScrollAreaPrimitive.Scrollbar
          key={orientation}
          orientation={orientation}
          className={cn(
            'bg-surface-sunken flex touch-none p-1 select-none',
            orientation === 'vertical' ? 'w-3 flex-col' : 'h-3 flex-row',
          )}
        >
          <ScrollAreaPrimitive.Thumb className="bg-control relative flex-1 rounded-full" />
        </ScrollAreaPrimitive.Scrollbar>
      ))}
    </ScrollAreaPrimitive.Root>
  );
}
