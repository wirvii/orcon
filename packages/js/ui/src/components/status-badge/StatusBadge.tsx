// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useId } from 'react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { Badge } from '../badge/Badge';

import { statusMap, type StatusKey } from './status-map';

export interface StatusBadgeProps {
  status: StatusKey;
  className?: string | undefined;
}

/**
 * Domain state as icon + text. Color only reinforces it. UNCERTAIN is never
 * drawn or named as a failure or as a success: it uses the attention family and
 * carries a fixed description for assistive technology.
 */
export function StatusBadge({ status, className }: StatusBadgeProps) {
  const intl = useIntl();
  const entry = statusMap[status];
  const descriptionId = useId();
  const Icon = entry.icon;
  return (
    <Badge
      tone={entry.family}
      data-status={status}
      aria-describedby={entry.description ? descriptionId : undefined}
      className={className}
      icon={<Icon aria-hidden className={cn('size-icon-16 shrink-0', entry.spins && 'motion-safe:animate-spin-slow')} />}
    >
      {intl.formatMessage(entry.label)}
      {entry.description ? (
        <span id={descriptionId} className="sr-only">
          {intl.formatMessage(entry.description)}
        </span>
      ) : null}
    </Badge>
  );
}
