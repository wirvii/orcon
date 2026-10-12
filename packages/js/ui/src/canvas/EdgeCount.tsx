// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Repeat } from 'lucide-react';
import { useIntl } from 'react-intl';

import { cn } from '../lib/cn';

import { canvasMessages } from './messages';

export interface EdgeCountProps {
  /** Items that travelled the connection in the execution being shown. */
  count: number;
  /** The connection goes back to an earlier node. */
  loopBack?: boolean;
  className?: string;
}

/** The chip on a connection after an execution: how many items went through, with a repeat icon on a loop back. */
export function EdgeCount({ count, loopBack = false, className }: EdgeCountProps) {
  const intl = useIntl();
  return (
    <span
      data-edge-count={count}
      className={cn(
        'bg-surface-raised text-fg-muted border-border inline-flex items-center gap-1 rounded-full border px-2 text-xs font-medium whitespace-nowrap shadow-sm',
        className,
      )}
    >
      {loopBack ? <Repeat aria-hidden className="size-icon-16 shrink-0" /> : null}
      {intl.formatMessage(canvasMessages.edgeItems, { count })}
    </span>
  );
}
