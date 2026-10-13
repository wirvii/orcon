// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { X } from 'lucide-react';
import { Dialog as DialogPrimitive } from 'radix-ui';
import type { ReactElement, ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { modalPanel, scrim } from '../../lib/control';
import { commonMessages } from '../../lib/messages';
import { IconButton } from '../icon-button/IconButton';

export interface DialogProps {
  open?: boolean | undefined;
  defaultOpen?: boolean | undefined;
  onOpenChange?: ((open: boolean) => void) | undefined;
  /** The button that opens the dialog. Omit it when `open` is controlled. */
  trigger?: ReactElement | undefined;
  /** Names the dialog. */
  title: string;
  description?: ReactNode | undefined;
  children?: ReactNode | undefined;
  /** Buttons at the bottom. The primary action goes last. */
  footer?: ReactNode | undefined;
  className?: string | undefined;
}

/**
 * A modal dialog. Focus moves inside and is trapped; Escape and the close
 * button dismiss it and focus returns to the trigger. For destructive
 * confirmations use AlertDialog.
 */
export function Dialog({ open, defaultOpen, onOpenChange, trigger, title, description, children, footer, className }: DialogProps) {
  const intl = useIntl();
  return (
    <DialogPrimitive.Root open={open} defaultOpen={defaultOpen} onOpenChange={onOpenChange}>
      {trigger ? <DialogPrimitive.Trigger asChild>{trigger}</DialogPrimitive.Trigger> : null}
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className={scrim} />
        <div className="pointer-events-none fixed inset-0 z-modal flex items-center justify-center p-4">
          <DialogPrimitive.Content
            {...(description ? {} : { 'aria-describedby': undefined })}
            className={cn(
              modalPanel,
              'pointer-events-auto flex max-h-full w-full max-w-screen-sm flex-col gap-3 overflow-y-auto rounded-xl p-4',
              className,
            )}
          >
            <div className="flex items-start justify-between gap-3">
              <DialogPrimitive.Title className="text-lg font-semibold">{title}</DialogPrimitive.Title>
              <DialogPrimitive.Close asChild>
                <IconButton variant="ghost" icon={X} label={intl.formatMessage(commonMessages.close)} />
              </DialogPrimitive.Close>
            </div>
            {description ? <DialogPrimitive.Description className="text-fg-muted">{description}</DialogPrimitive.Description> : null}
            {children}
            {footer ? <div className="flex flex-wrap justify-end gap-2 pt-1">{footer}</div> : null}
          </DialogPrimitive.Content>
        </div>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}

/** Closes the dialog it is placed in. Wrap a Button: <DialogClose><Button>…</Button></DialogClose>. */
export const DialogClose = DialogPrimitive.Close;
