// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, EmptyState, ProblemAlert, Skeleton } from '@orcon/ui';
import type { ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { Forbidden } from '../errors/Forbidden';
import { toProblemDetails } from '../errors/to-problem-details';

const messages = defineMessages({
  loading: {
    id: 'app.queryState.loading',
    defaultMessage: 'Loading',
    description: 'Accessible name of the region that shows placeholders while data loads.',
  },
  retry: {
    id: 'app.queryState.retry',
    defaultMessage: 'Try again',
    description: 'Button under an error that loads the data again.',
  },
});

/** The part of a Query result that {@link QueryState} reads, so a screen can pass `useQuery(...)` as it is. */
export interface QuerySource<TData> {
  readonly status: 'pending' | 'error' | 'success';
  readonly data: TData | undefined;
  readonly error: unknown;
  readonly refetch: () => unknown;
}

export interface QueryStateProps<TData> {
  query: QuerySource<TData>;
  /** True when the data holds nothing to show. Defaults to an empty array. */
  isEmpty?: ((data: TData) => boolean) | undefined;
  /** What an empty result says and, usually, the next step to take. */
  empty: {
    title: string;
    description?: string | undefined;
    action?: ReactNode;
    /** A decorative picture shown above the text on a first-run empty state (the mascot, SPEC-018). */
    illustration?: ReactNode | undefined;
  };
  /** Placeholder rows shown while loading. */
  skeletonRows?: number | undefined;
  /** The screen's content for loaded data. */
  children: (data: TData) => ReactNode;
}

/**
 * The one way a screen renders a query (orcon-web DT-8): placeholders while loading, an
 * empty state with a next step, an error with a retry, a permission message without a
 * retry, or the data. Every screen looks and behaves the same in each state.
 */
export function QueryState<TData>({ query, isEmpty = defaultIsEmpty, empty, skeletonRows = 3, children }: QueryStateProps<TData>) {
  const intl = useIntl();

  if (query.status === 'pending') {
    return (
      <div role="status" aria-busy="true" aria-label={intl.formatMessage(messages.loading)} className="flex flex-col gap-3">
        {Array.from({ length: skeletonRows }, (_, row) => (
          <Skeleton key={row} className="h-12 w-full" />
        ))}
      </div>
    );
  }

  if (query.status === 'error') {
    const problem = toProblemDetails(query.error);
    if (problem.status === 403) return <Forbidden />;
    return (
      <div className="flex flex-col items-start gap-3">
        <ProblemAlert problem={problem} />
        <Button
          variant="secondary"
          onClick={() => {
            void query.refetch();
          }}
        >
          {intl.formatMessage(messages.retry)}
        </Button>
      </div>
    );
  }

  const { data } = query;
  if (data === undefined || isEmpty(data)) {
    const state = <EmptyState title={empty.title} description={empty.description} action={empty.action} />;
    if (empty.illustration === undefined) return state;
    return (
      <div className="flex flex-col items-center gap-2">
        {empty.illustration}
        {state}
      </div>
    );
  }
  return <>{children(data)}</>;
}

function defaultIsEmpty(data: unknown): boolean {
  return Array.isArray(data) && data.length === 0;
}
