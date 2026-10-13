// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Tooltip as TooltipPrimitive } from 'radix-ui';
import type { ComponentProps, ReactElement, ReactNode } from 'react';

import { cn } from '../../lib/cn';

export interface TooltipProps {
  /** A short text that adds to what the trigger already says. It is never the only way to learn something. */
  content: ReactNode;
  /** The element that triggers the tooltip. It must be focusable, so keyboard users get the tooltip too. */
  children: ReactElement;
  side?: ComponentProps<typeof TooltipPrimitive.Content>['side'] | undefined;
  open?: boolean | undefined;
  defaultOpen?: boolean | undefined;
  onOpenChange?: ((open: boolean) => void) | undefined;
}

/** A hint shown on hover and on keyboard focus. Touch users do not get it, so never hide required information here. */
export function Tooltip({ content, children, side = 'top', open, defaultOpen, onOpenChange }: TooltipProps) {
  return (
    <TooltipPrimitive.Provider delayDuration={300}>
      <TooltipPrimitive.Root open={open} defaultOpen={defaultOpen} onOpenChange={onOpenChange}>
        <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
        <TooltipPrimitive.Portal>
          <TooltipPrimitive.Content
            side={side}
            sideOffset={6}
            className={cn('z-tooltip bg-surface-inverse text-fg-inverse max-w-screen-sm rounded-md px-2 py-1 text-sm')}
          >
            {content}
          </TooltipPrimitive.Content>
        </TooltipPrimitive.Portal>
      </TooltipPrimitive.Root>
    </TooltipPrimitive.Provider>
  );
}
