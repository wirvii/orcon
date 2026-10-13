// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { AlertDialog as AlertPrimitive } from 'radix-ui';
import type { ReactElement, ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { modalPanel, scrim } from '../../lib/control';
import { commonMessages } from '../../lib/messages';
import { Button } from '../button/Button';

export interface AlertDialogProps {
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
  trigger?: ReactElement;
  /** The question, for example "Delete this flow?". */
  title: string;
  /**
   * What will happen, in plain words: what is lost or changed and whether it can be
   * undone. Required; a destructive confirmation without impact text is not allowed.
   */
  impact: ReactNode;
  /** Names the action, for example "Delete flow". Never just "OK" or "Yes". */
  confirmLabel: string;
  /** Defaults to "Cancel". */
  cancelLabel?: string;
  onConfirm: () => void;
  /** Use the danger style for the confirm button. Default true. */
  destructive?: boolean;
}

/**
 * Confirms an irreversible or risky action. Focus starts on Cancel so a stray
 * Enter does not confirm. Escape cancels. Clicking outside does not dismiss it.
 */
export function AlertDialog({
  open,
  defaultOpen,
  onOpenChange,
  trigger,
  title,
  impact,
  confirmLabel,
  cancelLabel,
  onConfirm,
  destructive = true,
}: AlertDialogProps) {
  const intl = useIntl();
  return (
    <AlertPrimitive.Root open={open} defaultOpen={defaultOpen} onOpenChange={onOpenChange}>
      {trigger ? <AlertPrimitive.Trigger asChild>{trigger}</AlertPrimitive.Trigger> : null}
      <AlertPrimitive.Portal>
        <AlertPrimitive.Overlay className={scrim} />
        <div className="pointer-events-none fixed inset-0 z-modal flex items-center justify-center p-4">
          <AlertPrimitive.Content
            className={cn(modalPanel, 'pointer-events-auto flex max-h-full w-full max-w-screen-sm flex-col gap-3 overflow-y-auto rounded-xl p-4')}
          >
            <AlertPrimitive.Title className="text-lg font-semibold">{title}</AlertPrimitive.Title>
            <AlertPrimitive.Description className="text-fg-muted">{impact}</AlertPrimitive.Description>
            <div className="flex flex-wrap justify-end gap-2 pt-1">
              <AlertPrimitive.Cancel asChild>
                <Button variant="secondary">{cancelLabel ?? intl.formatMessage(commonMessages.cancel)}</Button>
              </AlertPrimitive.Cancel>
              <AlertPrimitive.Action asChild>
                <Button variant={destructive ? 'danger' : 'primary'} onClick={onConfirm}>
                  {confirmLabel}
                </Button>
              </AlertPrimitive.Action>
            </div>
          </AlertPrimitive.Content>
        </div>
      </AlertPrimitive.Portal>
    </AlertPrimitive.Root>
  );
}
