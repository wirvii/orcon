// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Popover as PopoverPrimitive } from 'radix-ui';
import type { ComponentProps, ReactElement, ReactNode } from 'react';

import { cn } from '../../lib/cn';
import { floatingPanel } from '../../lib/control';

export interface PopoverProps extends Pick<ComponentProps<typeof PopoverPrimitive.Root>, 'open' | 'defaultOpen' | 'onOpenChange'> {
  /** The element that opens the popover. It must be a button or another focusable control. */
  trigger: ReactElement;
  /** Accessible name of the popover panel. */
  label: string;
  children: ReactNode;
  side?: ComponentProps<typeof PopoverPrimitive.Content>['side'] | undefined;
  align?: ComponentProps<typeof PopoverPrimitive.Content>['align'] | undefined;
  className?: string | undefined;
}

/**
 * A panel anchored to its trigger. Focus moves into it when it opens and
 * returns to the trigger on Escape. For a choice among options use Select or
 * DropdownMenu; for a decision that blocks the page use Dialog.
 */
export function Popover({ trigger, label, children, side = 'bottom', align = 'start', className, ...root }: PopoverProps) {
  return (
    <PopoverPrimitive.Root {...root}>
      <PopoverPrimitive.Trigger asChild>{trigger}</PopoverPrimitive.Trigger>
      <PopoverPrimitive.Portal>
        <PopoverPrimitive.Content
          aria-label={label}
          side={side}
          align={align}
          sideOffset={4}
          collisionPadding={8}
          className={cn(floatingPanel, 'w-panel-width max-h-available overflow-y-auto p-3', className)}
        >
          {children}
        </PopoverPrimitive.Content>
      </PopoverPrimitive.Portal>
    </PopoverPrimitive.Root>
  );
}
