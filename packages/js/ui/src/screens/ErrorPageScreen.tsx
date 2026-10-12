// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, useIntl } from 'react-intl';

import { BrandMark } from '../components/brand-mark/BrandMark';
import { Button } from '../components/button/Button';
import { Link } from '../components/link/Link';
import { ProblemAlert } from '../components/problem-alert/ProblemAlert';
import type { ProblemDetails } from '../components/problem-alert/problem';

const messages = defineMessages({
  title: { id: 'screen.error.title', defaultMessage: 'Something went wrong', description: 'Main heading of the page shown when the server could not do what was asked.' },
  text: {
    id: 'screen.error.text',
    defaultMessage: 'Nothing you entered was lost. Try again. If it keeps happening, give the request number below to an administrator.',
    description: 'Explains the error page: nothing was lost, what to do and what to report.',
  },
  retry: { id: 'screen.error.retry', defaultMessage: 'Try again', description: 'Button on the error page that repeats the request.' },
  home: { id: 'screen.error.home', defaultMessage: 'Go to flows', description: 'Link on the error page that goes to the list of flows.' },
});

const exampleProblem: ProblemDetails = {
  status: 500,
  title: 'The server could not finish the request',
  code: 'internal-error',
  detail: 'upstream timeout after 30s while reading the flow definition',
  requestId: 'req_5b21c9e0',
};

export interface ErrorPageScreenProps {
  problem?: ProblemDetails;
  onRetry?: () => void;
}

/**
 * S8 (part 3): the error page. There is no mascot on an error: the page says what
 * happened, that nothing was lost, and shows the request number to report.
 */
export function ErrorPageScreen({ problem = exampleProblem, onRetry }: ErrorPageScreenProps) {
  const intl = useIntl();
  return (
    <div className="bg-app text-fg flex min-h-screen flex-col">
      <header className="bg-surface border-border flex h-row min-h-target items-center border-b px-4">
        <BrandMark />
      </header>
      <main className="mx-auto flex w-full max-w-screen-md flex-1 flex-col gap-4 p-4">
        <h1 className="text-2xl font-semibold break-words">{intl.formatMessage(messages.title)}</h1>
        <p className="text-fg-muted break-words">{intl.formatMessage(messages.text)}</p>
        <ProblemAlert problem={problem} />
        <div className="flex flex-wrap items-center gap-3">
          <Button onClick={onRetry}>{intl.formatMessage(messages.retry)}</Button>
          <Link href="#flows" className="inline-flex min-h-target items-center">
            {intl.formatMessage(messages.home)}
          </Link>
        </div>
      </main>
    </div>
  );
}
