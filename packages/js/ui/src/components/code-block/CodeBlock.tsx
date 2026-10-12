// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Check, Copy } from 'lucide-react';
import { useEffect, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { IconButton } from '../icon-button/IconButton';

const messages = defineMessages({
  copy: { id: 'codeBlock.copy', defaultMessage: 'Copy code', description: 'Accessible name of the button that copies the contents of a code block.' },
  copied: { id: 'codeBlock.copied', defaultMessage: 'Copied', description: 'Confirmation announced after copying a code block.' },
});

export interface CodeBlockProps {
  code: string;
  /** Names the block for assistive technology, for example "Request body". */
  label: string;
  /** Show a copy button. Never use it for values the person may not see; use ProtectedValue for those. */
  copyable?: boolean;
  className?: string;
}

/**
 * Monospace code, expressions, ids or JSON. Long lines scroll sideways inside the
 * block; the block is focusable so keyboard users can scroll it.
 */
export function CodeBlock({ code, label, copyable = false, className }: CodeBlockProps) {
  const intl = useIntl();
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 3000);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const copy = async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
  };

  return (
    <div className={cn('bg-surface-sunken relative rounded-md', className)}>
      <pre
        role="region"
        aria-label={label}
        tabIndex={0}
        className={cn('text-fg overflow-x-auto rounded-md p-3 font-mono text-mono', copyable && 'pr-12')}
      >
        <code>{code}</code>
      </pre>
      {copyable ? (
        <div className="absolute top-1 right-1">
          <IconButton variant="ghost" icon={copied ? Check : Copy} label={intl.formatMessage(messages.copy)} onClick={copy} />
        </div>
      ) : null}
      <span role="status" className="sr-only">
        {copied ? intl.formatMessage(messages.copied) : null}
      </span>
    </div>
  );
}
