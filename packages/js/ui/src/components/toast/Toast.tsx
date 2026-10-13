// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { CircleAlert, CircleCheck, CircleX, Info, TriangleAlert } from 'lucide-react';
import { Toaster as Sonner, toast } from 'sonner';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  region: {
    id: 'common.toast.region',
    defaultMessage: 'Notifications',
    description: 'Accessible name of the region where short confirmation messages appear.',
  },
});

/**
 * Where toasts appear. Render it once near the root of the app.
 *
 * Toasts confirm something that already happened ("Flow saved") and disappear on
 * their own. They are announced politely and never take focus. Do not use a toast
 * for anything the person must read, decide or act on: use a Banner or a Dialog.
 */
export interface ToasterProps {
  /** How many toasts show at once; older ones wait behind. Defaults to 3. */
  visibleToasts?: number;
}

export function Toaster({ visibleToasts }: ToasterProps) {
  const intl = useIntl();
  return (
    <Sonner
      position="bottom-right"
      closeButton
      expand
      visibleToasts={visibleToasts}
      containerAriaLabel={intl.formatMessage(messages.region)}
      icons={{
        success: <CircleCheck aria-hidden className="size-icon-16" />,
        info: <Info aria-hidden className="size-icon-16" />,
        warning: <TriangleAlert aria-hidden className="size-icon-16" />,
        error: <CircleX aria-hidden className="size-icon-16" />,
        loading: <CircleAlert aria-hidden className="size-icon-16" />,
      }}
      toastOptions={{
        unstyled: true,
        classNames: {
          toast:
            'z-toast flex w-full items-start gap-2 rounded-lg border border-border bg-surface-raised p-3 text-base text-fg shadow-md',
          title: 'font-medium',
          description: 'text-fg-muted',
          icon: 'mt-0',
          success: 'text-status-success',
          warning: 'text-status-warning',
          error: 'text-status-danger',
          actionButton: 'ml-auto inline-flex h-control min-h-target items-center rounded-md border border-control px-control-x text-fg',
          closeButton: 'inline-flex size-icon-24 min-h-target min-w-target items-center justify-center rounded-md text-fg',
        },
      }}
    />
  );
}

export { toast };
