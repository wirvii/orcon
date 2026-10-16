// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Hourglass, type LucideIcon } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { Badge } from '../badge/Badge';

const messages = defineMessages({
  label: {
    id: 'comingSoon.label',
    defaultMessage: 'Coming soon',
    description: 'Word shown on a feature whose server part does not exist yet.',
  },
});

export interface ComingSoonProps {
  /** What is not available yet, in the person's words. */
  title: string;
  /** One sentence that says what is missing. */
  description: string;
  /** The icon of the feature. A neutral hourglass when omitted. */
  icon?: LucideIcon | undefined;
  /** Heading level of the title, to keep the page outline correct. */
  headingLevel?: 2 | 3 | 4 | undefined;
  className?: string | undefined;
}

/**
 * A feature whose backend does not exist yet: an icon, the words "Coming soon", a title and one sentence that
 * says what is missing. It has no illustration and no action, because there is nothing to do yet. It is never
 * an error: nothing failed.
 */
export function ComingSoon({ title, description, icon: Icon = Hourglass, headingLevel = 2, className }: ComingSoonProps) {
  const intl = useIntl();
  const Heading = `h${headingLevel}` as const;
  return (
    <div
      data-coming-soon
      className={cn('border-strong bg-surface flex flex-col items-start gap-3 rounded-lg border border-dashed p-5 wrap-anywhere', className)}
    >
      <span className="bg-status-neutral-bg text-status-neutral inline-flex size-10 items-center justify-center rounded-md">
        <Icon aria-hidden className="size-icon-20" />
      </span>
      <Badge tone="neutral">{intl.formatMessage(messages.label)}</Badge>
      <div className="flex flex-col gap-1">
        <Heading className="text-fg text-md font-semibold">{title}</Heading>
        <p className="text-fg-muted max-w-screen-sm">{description}</p>
      </div>
    </div>
  );
}
