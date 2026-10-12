// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react';

import { cn } from '../../lib/cn';

export interface EmptyStateProps {
  title: string;
  description?: ReactNode;
  /** The next step, usually a Button. An empty state always says what to do. */
  action?: ReactNode;
  /**
   * Optional illustration slot. Pass the Mascot only in an allowed context
   * (first-run welcome, "nothing here yet"); never in errors or dense views.
   */
  mascot?: ReactNode;
  /** Heading level of the title, to keep the page outline correct. */
  headingLevel?: 2 | 3 | 4;
  className?: string;
}

export function EmptyState({ title, description, action, mascot, headingLevel = 2, className }: EmptyStateProps) {
  const Heading = `h${headingLevel}` as 'h2' | 'h3' | 'h4';
  return (
    <div className={cn('flex flex-col items-center gap-3 px-4 py-8 text-center break-words', className)}>
      {mascot}
      <Heading className="text-fg text-lg font-semibold">{title}</Heading>
      {description ? <p className="text-fg-muted max-w-screen-sm">{description}</p> : null}
      {action ? <div className="mt-1 flex flex-wrap justify-center gap-2">{action}</div> : null}
    </div>
  );
}
