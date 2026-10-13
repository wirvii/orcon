// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { X } from 'lucide-react';
import { Dialog as SheetPrimitive } from 'radix-ui';
import type { ReactElement, ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { modalPanel, scrim } from '../../lib/control';
import { commonMessages } from '../../lib/messages';
import { IconButton } from '../icon-button/IconButton';

export interface SheetProps {
  open?: boolean | undefined;
  defaultOpen?: boolean | undefined;
  onOpenChange?: ((open: boolean) => void) | undefined;
  trigger?: ReactElement | undefined;
  title: string;
  description?: ReactNode | undefined;
  children?: ReactNode | undefined;
  /**
   * `responsive` (default): a bottom sheet below the md breakpoint (phones) and a
   * side panel from md up. `bottom` and `right` fix the placement.
   */
  side?: 'responsive' | 'bottom' | 'right' | undefined;
}

const placement = {
  responsive: 'inset-x-0 bottom-0 rounded-t-xl md:inset-y-0 md:right-0 md:left-auto md:max-w-screen-sm md:rounded-none md:rounded-l-xl',
  bottom: 'inset-x-0 bottom-0 rounded-t-xl',
  right: 'inset-y-0 right-0 left-auto max-w-screen-sm rounded-l-xl',
} as const;

/**
 * A panel that slides in from an edge: a bottom sheet on phones, a side panel on
 * larger screens. It behaves like a modal dialog (focus trap, Escape, focus return).
 */
export function Sheet({ open, defaultOpen, onOpenChange, trigger, title, description, children, side = 'responsive' }: SheetProps) {
  const intl = useIntl();
  return (
    <SheetPrimitive.Root open={open} defaultOpen={defaultOpen} onOpenChange={onOpenChange}>
      {trigger ? <SheetPrimitive.Trigger asChild>{trigger}</SheetPrimitive.Trigger> : null}
      <SheetPrimitive.Portal>
        <SheetPrimitive.Overlay className={scrim} />
        <SheetPrimitive.Content
          {...(description ? {} : { 'aria-describedby': undefined })}
          className={cn(modalPanel, 'fixed flex max-h-full w-full flex-col gap-3 overflow-y-auto p-4', placement[side])}
        >
          <div className="flex items-start justify-between gap-3">
            <SheetPrimitive.Title className="text-lg font-semibold">{title}</SheetPrimitive.Title>
            <SheetPrimitive.Close asChild>
              <IconButton variant="ghost" icon={X} label={intl.formatMessage(commonMessages.close)} />
            </SheetPrimitive.Close>
          </div>
          {description ? <SheetPrimitive.Description className="text-fg-muted">{description}</SheetPrimitive.Description> : null}
          {children}
        </SheetPrimitive.Content>
      </SheetPrimitive.Portal>
    </SheetPrimitive.Root>
  );
}
