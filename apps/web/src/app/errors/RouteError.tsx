// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, ProblemAlert } from '@orcon/ui';
import { useRouter, type ErrorComponentProps } from '@tanstack/react-router';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { ContractError } from './contract-error';
import { Forbidden } from './Forbidden';
import { reportContractError } from './report';
import { toProblemDetails } from './to-problem-details';
import { NotFound } from '../shell/NotFound';

const messages = defineMessages({
  retry: {
    id: 'app.error.retry',
    defaultMessage: 'Try again',
    description: 'Button under an error that repeats the request that failed.',
  },
});

/**
 * The error component of every route (orcon-web RT-7, ER-3): a permission error says so and
 * offers no retry, a missing resource shows the not-found page, anything else shows the
 * problem with a retry. A response that broke the contract shows the generic message and is
 * logged without its content (ER-5).
 */
export function RouteError({ error, reset }: ErrorComponentProps) {
  const intl = useIntl();
  const router = useRouter();
  const problem = toProblemDetails(error);

  useEffect(() => {
    if (error instanceof ContractError) reportContractError(error, problem.requestId);
  }, [error, problem.requestId]);

  if (problem.status === 403) return <Forbidden />;
  if (problem.status === 404) return <NotFound />;

  return (
    <div className="flex flex-col items-start gap-3">
      <ProblemAlert problem={problem} />
      <Button
        variant="secondary"
        onClick={() => {
          void router.invalidate();
          reset();
        }}
      >
        {intl.formatMessage(messages.retry)}
      </Button>
    </div>
  );
}
