// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Check, Copy, Lock } from 'lucide-react';
import { useEffect, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { IconButton } from '../icon-button/IconButton';

const messages = defineMessages({
  hidden: {
    id: 'redacted.hidden',
    defaultMessage: 'Hidden — you need permission to view execution data',
    description: 'Shown in place of execution data the person is not allowed to see. Never empty.',
  },
  copyValue: { id: 'redacted.copyValue', defaultMessage: 'Copy value', description: 'Accessible name of the button that copies a visible value.' },
  copied: { id: 'redacted.copied', defaultMessage: 'Copied', description: 'Confirmation shown after copying a value.' },
});

/** Stands in for a value the person may not see. It says so; it is never blank. */
export function RedactedValue({ className }: { className?: string }) {
  const intl = useIntl();
  return (
    <span
      data-redacted
      className={cn(
        'bg-surface-sunken text-fg-muted inline-flex items-center gap-1 rounded-sm px-2 font-mono text-mono',
        className,
      )}
    >
      <Lock aria-hidden className="size-icon-16 shrink-0" />
      {intl.formatMessage(messages.hidden)}
    </span>
  );
}

export interface ProtectedValueProps {
  /** The real value. It is not rendered, not copyable and not placed in the page when `hidden` is true. */
  value: string;
  /** The person lacks `execution:read-data`. */
  hidden: boolean;
  className?: string;
}

/**
 * A value that may be protected. Visible: the value and a copy button. Hidden:
 * only the RedactedValue notice; the value never reaches the DOM or the clipboard.
 */
export function ProtectedValue({ value, hidden, className }: ProtectedValueProps) {
  const intl = useIntl();
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 3000);
    return () => window.clearTimeout(timer);
  }, [copied]);

  if (hidden) return <RedactedValue className={className} />;

  const copy = async () => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
  };

  return (
    <span className={cn('inline-flex items-center gap-1', className)}>
      <code className="bg-surface-sunken text-fg rounded-sm px-2 font-mono text-mono break-all">{value}</code>
      <IconButton variant="ghost" icon={copied ? Check : Copy} label={intl.formatMessage(messages.copyValue)} onClick={copy} />
      <span role="status" className="sr-only">
        {copied ? intl.formatMessage(messages.copied) : null}
      </span>
    </span>
  );
}
