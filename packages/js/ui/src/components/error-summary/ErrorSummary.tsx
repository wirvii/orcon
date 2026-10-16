// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { CircleAlert } from 'lucide-react';
import { useEffect, useRef, type MouseEvent } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';

const messages = defineMessages({
  title: {
    id: 'common.errorSummary.title',
    defaultMessage: '{count, plural, one {There is # problem to fix} other {There are # problems to fix}}',
    description: 'Heading of the summary at the top of a form that lists the fields with errors. # is the number of errors.',
  },
});

export interface FieldError {
  /** Id of the control (the FormField `id`). The summary links to it. */
  fieldId: string;
  message: string;
}

export interface ErrorSummaryProps {
  errors: readonly FieldError[];
  /** Move focus to the summary when it appears, so a screen reader announces it. Use after a failed submit. */
  focusOnMount?: boolean | undefined;
  className?: string | undefined;
}

/** Lists every field error at the top of a form. Each entry moves focus to its field. */
export function ErrorSummary({ errors, focusOnMount = false, className }: ErrorSummaryProps) {
  const intl = useIntl();
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (focusOnMount) ref.current?.focus();
  }, [focusOnMount]);

  if (errors.length === 0) return null;

  const focusField = (event: MouseEvent<HTMLAnchorElement>, fieldId: string) => {
    const control = document.getElementById(fieldId);
    if (control) {
      event.preventDefault();
      control.focus();
      control.scrollIntoView({ block: 'center' });
    }
  };

  return (
    <div
      ref={ref}
      role="alert"
      tabIndex={-1}
      className={cn('border-status-danger bg-status-danger-bg text-status-danger rounded-md border p-3 wrap-anywhere', className)}
    >
      <h2 className="flex items-center gap-2 text-base font-semibold">
        <CircleAlert aria-hidden className="size-icon-20 shrink-0" />
        <span className="min-w-0">{intl.formatMessage(messages.title, { count: errors.length })}</span>
      </h2>
      <ul className="mt-2 flex list-none flex-col gap-1 pl-0">
        {errors.map((error) => (
          <li key={error.fieldId}>
            <a
              href={`#${error.fieldId}`}
              onClick={(event) => focusField(event, error.fieldId)}
              className="hover:text-fg inline-flex min-h-target items-center rounded-sm font-medium"
            >
              {error.message}
            </a>
          </li>
        ))}
      </ul>
    </div>
  );
}
