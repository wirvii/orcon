// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { BellRing, Clock, TriangleAlert } from 'lucide-react';
import { useId } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { Badge } from '../badge/Badge';
import { Checkbox } from '../checkbox/Checkbox';
import { Link } from '../link/Link';
import { RelativeTime } from '../relative-time/RelativeTime';
import type { DateInput } from '../date-time/DateTime';

const messages = defineMessages({
  select: {
    id: 'inbox.select',
    defaultMessage: 'Select {title}',
    description: 'Accessible name of the checkbox that selects one item of the attention inbox for a batch action. {title} is the cause of the item.',
  },
  escalation: {
    id: 'inbox.escalationLevel',
    defaultMessage: 'Escalation level {level}',
    description: 'Badge on an attention item that was escalated. {level} is the escalation level, starting at 1.',
  },
  acknowledgedBy: {
    id: 'inbox.acknowledgedBy',
    defaultMessage: 'Acknowledged by {name}',
    description: 'Says who acknowledged an attention item. {name} is the person.',
  },
  overdue: { id: 'inbox.overdue', defaultMessage: 'Overdue', description: 'Badge on an attention item whose due time has passed.' },
  waiting: { id: 'inbox.waiting', defaultMessage: 'Waiting', description: 'Label before how long an attention item has been waiting.' },
});

export interface AttentionInboxItemProps {
  /** The cause: what needs a person, in a short sentence. */
  title: string;
  /** Link to the detail where the person decides. */
  href: string;
  /** The flow the item belongs to. Batch actions only group items with the same cause and flow. */
  flowName: string;
  createdAt: DateInput;
  /** The instant "now" is measured from. Pass it in stories and tests. */
  now?: DateInput;
  /** Escalation level, starting at 1. Omit when the item was not escalated. */
  escalationLevel?: number;
  /** Who acknowledged it. Omit when nobody did. */
  acknowledgedBy?: string;
  overdue?: boolean;
  selected?: boolean;
  onSelectedChange?: (selected: boolean) => void;
  className?: string;
}

/**
 * One row of the human-attention inbox: cause, flow, age, escalation, who
 * acknowledged it and whether it is overdue. State is shown with icons and text.
 * No mascot belongs in this dense view.
 */
export function AttentionInboxItem({
  title,
  href,
  flowName,
  createdAt,
  now,
  escalationLevel,
  acknowledgedBy,
  overdue = false,
  selected = false,
  onSelectedChange,
  className,
}: AttentionInboxItemProps) {
  const intl = useIntl();
  const titleId = useId();
  return (
    <article
      aria-labelledby={titleId}
      data-state={selected ? 'selected' : undefined}
      className={cn(
        'border-border flex items-start gap-2 border-b px-2 py-2 text-base break-words',
        selected && 'bg-surface-sunken',
        className,
      )}
    >
      {onSelectedChange ? (
        <Checkbox
          checked={selected}
          onCheckedChange={(next) => onSelectedChange(next === true)}
          label={<span className="sr-only">{intl.formatMessage(messages.select, { title })}</span>}
          className="w-auto min-w-target justify-center px-0"
        />
      ) : null}
      <BellRing aria-hidden className="text-status-attention mt-3 size-icon-20 shrink-0" />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <h3 id={titleId} className="text-fg font-medium">
          <Link href={href} className="text-fg-link">
            {title}
          </Link>
        </h3>
        <p className="text-fg-muted flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
          <span>{flowName}</span>
          <span className="inline-flex items-center gap-1">
            <Clock aria-hidden className="size-icon-16" />
            <span className="sr-only">{intl.formatMessage(messages.waiting)}: </span>
            <RelativeTime value={createdAt} now={now} />
          </span>
          {acknowledgedBy ? <span>{intl.formatMessage(messages.acknowledgedBy, { name: acknowledgedBy })}</span> : null}
        </p>
        {escalationLevel !== undefined || overdue ? (
          <p className="flex flex-wrap items-center gap-2">
            {escalationLevel !== undefined ? (
              <Badge tone="attention">{intl.formatMessage(messages.escalation, { level: escalationLevel })}</Badge>
            ) : null}
            {overdue ? (
              <Badge tone="danger" icon={<TriangleAlert aria-hidden className="size-icon-16" />}>
                {intl.formatMessage(messages.overdue)}
              </Badge>
            ) : null}
          </p>
        ) : null}
      </div>
    </article>
  );
}
