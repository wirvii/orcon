// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { CircleX, Lightbulb } from 'lucide-react';
import type { ReactNode } from 'react';
import { useId } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { CodeBlock } from '../code-block/CodeBlock';

const messages = defineMessages({
  original: {
    id: 'plainError.original',
    defaultMessage: 'Original error',
    description: 'Label of the block that shows the error exactly as the service or the engine reported it, under the explanation in plain words.',
  },
  fix: {
    id: 'plainError.fix',
    defaultMessage: 'How to fix it',
    description: 'Heading of the suggestion of what to change to make the error go away.',
  },
});

export interface PlainErrorProps {
  /** What went wrong, in one plain sentence. */
  title: string;
  /** Why, in plain words. */
  detail?: string | undefined;
  /** The error as the system reported it. It is shown as given, in a block of its own, never rewritten. */
  original?: string | undefined;
  /** Accessible name of the original error block. Defaults to "Original error". */
  originalLabel?: string | undefined;
  /**
   * A suggestion, only when the connector declares one for this error. The component never makes one up, and it has no
   * button that applies it: applying a fix in the editor does not exist yet.
   */
  fix?: ReactNode;
  /** Heading level of the title, to keep the page outline correct. */
  headingLevel?: 2 | 3 | 4 | undefined;
  className?: string | undefined;
}

/**
 * An error explained in plain words, with the original error next to it. The title says what happened, the detail
 * says why, and the original stays available for support and for people who know the service. The icon and the
 * words carry the meaning; red only reinforces them.
 */
export function PlainError({ title, detail, original, originalLabel, fix, headingLevel = 3, className }: PlainErrorProps) {
  const intl = useIntl();
  const titleId = useId();
  const Heading = `h${headingLevel}` as const;
  return (
    <section aria-labelledby={titleId} data-plain-error className={cn('flex flex-col gap-3', className)}>
      <div className="bg-status-danger-bg text-status-danger flex items-start gap-3 rounded-lg p-4">
        <CircleX aria-hidden className="size-icon-20 mt-1 shrink-0" />
        <div className="flex min-w-0 flex-col gap-1">
          <Heading id={titleId} className="text-md font-semibold wrap-anywhere">
            {title}
          </Heading>
          {detail ? <p className="wrap-anywhere">{detail}</p> : null}
        </div>
      </div>
      {original ? <CodeBlock label={originalLabel ?? intl.formatMessage(messages.original)} code={original} /> : null}
      {fix ? (
        <div className="border-border bg-surface flex flex-col gap-2 rounded-lg border p-4" data-plain-error-fix>
          <p className="text-fg flex items-center gap-2 font-semibold">
            <Lightbulb aria-hidden className="size-icon-16" />
            {intl.formatMessage(messages.fix)}
          </p>
          <div className="text-fg wrap-anywhere">{fix}</div>
        </div>
      ) : null}
    </section>
  );
}
