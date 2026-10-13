// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { LoaderCircle } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';

const messages = defineMessages({
  loading: {
    id: 'common.spinner.loading',
    defaultMessage: 'Loading',
    description: 'Accessible name of a spinner that shows something is loading.',
  },
});

export interface SpinnerProps {
  /** Icon size in px: 16, 20 or 24. */
  size?: 16 | 20 | 24 | undefined;
  /** Decorative spinners (inside a button that already says what is happening) are hidden from assistive tech. */
  decorative?: boolean | undefined;
  className?: string | undefined;
}

const sizeClass = { 16: 'size-icon-16', 20: 'size-icon-20', 24: 'size-icon-24' } as const;

/** Indeterminate progress. Rotation stops under reduced motion; the label still says "Loading". */
export function Spinner({ size = 16, decorative = false, className }: SpinnerProps) {
  const intl = useIntl();
  return (
    <LoaderCircle
      aria-hidden={decorative || undefined}
      role={decorative ? undefined : 'img'}
      aria-label={decorative ? undefined : intl.formatMessage(messages.loading)}
      className={cn('shrink-0 text-fg-muted motion-safe:animate-spin-slow', sizeClass[size], className)}
    />
  );
}
