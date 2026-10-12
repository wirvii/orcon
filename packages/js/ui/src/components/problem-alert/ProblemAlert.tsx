// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Copy } from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { Banner } from '../banner/Banner';
import { IconButton } from '../icon-button/IconButton';

import { knownProblemMessages, problemMessages, type ProblemDetails } from './problem';

const messages = defineMessages({
  technicalDetails: {
    id: 'problem.technicalDetails',
    defaultMessage: 'Technical details',
    description: 'Label of the collapsible section with the raw details of a server error.',
  },
  requestId: {
    id: 'problem.requestId',
    defaultMessage: 'Request ID',
    description: 'Label of the identifier a person can give to support to find a failed request.',
  },
  copyRequestId: {
    id: 'problem.copyRequestId',
    defaultMessage: 'Copy request ID',
    description: 'Accessible name of the button that copies the request identifier.',
  },
  copied: { id: 'problem.copied', defaultMessage: 'Copied', description: 'Confirmation shown after copying to the clipboard.' },
  status: { id: 'problem.status', defaultMessage: 'Status', description: 'Label of the HTTP status in the technical details.' },
  code: { id: 'problem.code', defaultMessage: 'Code', description: 'Label of the error code in the technical details.' },
  type: { id: 'problem.type', defaultMessage: 'Type', description: 'Label of the error type in the technical details.' },
  instance: { id: 'problem.instance', defaultMessage: 'Instance', description: 'Label of the error instance in the technical details.' },
  detail: { id: 'problem.detail', defaultMessage: 'Detail', description: 'Label of the error detail text in the technical details.' },
});

export interface ProblemAlertProps {
  problem: ProblemDetails;
  /**
   * For `attention-already-resolved` (HTTP 409): the outcome that is now in
   * effect, so the person sees the result instead of a generic error.
   */
  currentResult?: ReactNode;
  className?: string;
}

/**
 * Shows a server error from `application/problem+json`. The message comes from
 * the catalog by `code`; without a translation it falls back to `title`. The raw
 * `detail` stays in a collapsible "Technical details" section and the
 * `requestId` can be copied. Field errors are not shown here: map them to
 * FormField with `problemFieldErrors` and ErrorSummary.
 */
export function ProblemAlert({ problem, currentResult, className }: ProblemAlertProps) {
  const intl = useIntl();
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 3000);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const alreadyResolved = problem.code === 'attention-already-resolved';
  const known = problem.code ? knownProblemMessages[problem.code] : undefined;
  const message = known ? intl.formatMessage(known) : (problem.title ?? intl.formatMessage(problemMessages.generic));

  const copy = async () => {
    if (!problem.requestId) return;
    await navigator.clipboard.writeText(problem.requestId);
    setCopied(true);
  };

  const rows: [string, string | number | undefined][] = [
    [intl.formatMessage(messages.status), problem.status],
    [intl.formatMessage(messages.code), problem.code],
    [intl.formatMessage(messages.type), problem.type],
    [intl.formatMessage(messages.instance), problem.instance],
  ];
  const hasDetails = Boolean(problem.detail) || rows.some(([, value]) => value !== undefined);

  return (
    <Banner tone={alreadyResolved ? 'info' : 'danger'} title={message} className={className} data-problem-code={problem.code}>
      {alreadyResolved && currentResult ? <div>{currentResult}</div> : null}
      {hasDetails ? (
        <details className="mt-1">
          <summary className="inline-flex min-h-target cursor-pointer items-center rounded-sm underline underline-offset-2">
            {intl.formatMessage(messages.technicalDetails)}
          </summary>
          <dl className="mt-1 flex flex-col gap-1 font-mono text-mono">
            {(problem.detail ? [[intl.formatMessage(messages.detail), problem.detail] as const, ...rows] : rows)
              .filter(([, value]) => value !== undefined)
              .map(([label, value]) => (
                <div key={label} className="flex gap-3">
                  <dt className="shrink-0 font-medium">{label}</dt>
                  <dd className="min-w-0 break-words">{value}</dd>
                </div>
              ))}
          </dl>
        </details>
      ) : null}
      {problem.requestId ? (
        <div className="mt-1 flex flex-wrap items-center gap-2">
          <span>{intl.formatMessage(messages.requestId)}</span>
          <code className="font-mono text-mono break-all">{problem.requestId}</code>
          <IconButton variant="secondary" icon={Copy} label={intl.formatMessage(messages.copyRequestId)} onClick={copy} />
          <span role="status" className="text-sm">
            {copied ? intl.formatMessage(messages.copied) : null}
          </span>
        </div>
      ) : null}
    </Banner>
  );
}
