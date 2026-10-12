// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ShieldQuestion } from 'lucide-react';
import { useId } from 'react';
import { useIntl } from 'react-intl';

import { cn } from '../lib/cn';
import { statusMap, type StatusFamily, type StatusKey } from '../components/status-badge/status-map';

import { canvasMessages } from './messages';

const text: Record<StatusFamily, string> = {
  neutral: 'text-status-neutral',
  active: 'text-status-active',
  success: 'text-status-success',
  warning: 'text-status-warning',
  danger: 'text-status-danger',
  attention: 'text-status-attention',
};

export interface NodeStatusProps {
  status: StatusKey;
  /** Items of the node that are UNCERTAIN. Shown as a second line in the attention family. */
  uncertainCount?: number;
  className?: string;
}

/**
 * The execution state of a node, as icon + text on the node itself. Color only
 * reinforces it. UNCERTAIN keeps its fixed description and never uses the
 * failure or success families.
 */
export function NodeStatus({ status, uncertainCount = 0, className }: NodeStatusProps) {
  const intl = useIntl();
  const entry = statusMap[status];
  const descriptionId = useId();
  const Icon = entry.icon;
  return (
    <div className={cn('flex flex-col gap-1', className)} data-node-status={status}>
      <span
        className={cn('inline-flex items-center gap-1 text-xs font-medium', text[entry.family])}
        aria-describedby={entry.description ? descriptionId : undefined}
      >
        <Icon aria-hidden className={cn('size-icon-16 shrink-0', entry.spins && 'motion-safe:animate-spin-slow')} />
        <span>{intl.formatMessage(entry.label)}</span>
        {entry.description ? (
          <span id={descriptionId} className="sr-only">
            {intl.formatMessage(entry.description)}
          </span>
        ) : null}
      </span>
      {uncertainCount > 0 && status !== 'UNCERTAIN' ? (
        <span className={cn('inline-flex items-center gap-1 text-xs font-medium', text.attention)} data-uncertain-count={uncertainCount}>
          <ShieldQuestion aria-hidden className="size-icon-16 shrink-0" />
          <span>{intl.formatMessage(canvasMessages.uncertainItems, { count: uncertainCount })}</span>
        </span>
      ) : null}
    </div>
  );
}
