// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Bell, BellRing } from 'lucide-react';
import type { Ref } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { buttonVariants } from '../button/Button';

const messages = defineMessages({
  label: {
    id: 'bell.label',
    defaultMessage: '{count, plural, =0 {Nothing needs your attention} one {# item needs your attention} other {# items need your attention}}',
    description: 'Accessible name of the bell button. {count} is how many items wait for a person.',
  },
});

export interface NotificationBellProps {
  ref?: Ref<HTMLButtonElement>;
  /** Items that need a person's attention. */
  count: number;
  /** Opens the attention inbox. */
  onClick?: () => void;
  className?: string;
}

/** The bell in the top bar. Its name carries the count in words; the number on the badge is the same count, so the visible text is part of the name. */
export function NotificationBell({ count, onClick, className, ref }: NotificationBellProps) {
  const intl = useIntl();
  const Icon = count > 0 ? BellRing : Bell;
  return (
    <button
      ref={ref}
      type="button"
      onClick={onClick}
      aria-label={intl.formatMessage(messages.label, { count })}
      className={cn(buttonVariants({ variant: 'ghost', size: 'icon' }), 'relative', className)}
    >
      <Icon aria-hidden className="size-icon-20" />
      {count > 0 ? (
        <span
          aria-hidden
          className="bg-status-attention-bg text-status-attention border-status-attention absolute -top-1 -right-1 inline-flex min-w-icon-20 items-center justify-center rounded-full border px-1 text-xs font-semibold"
        >
          {count}
        </span>
      ) : null}
    </button>
  );
}
