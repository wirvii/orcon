// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Component, type ComponentType, type ErrorInfo, type ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { Banner } from '../banner/Banner';
import { Button } from '../button/Button';

const messages = defineMessages({
  title: {
    id: 'common.errorBoundary.title',
    defaultMessage: 'Something went wrong on this screen.',
    description: 'Heading of the fallback shown when a screen crashes.',
  },
  hint: {
    id: 'common.errorBoundary.hint',
    defaultMessage: 'Your data was not changed. Try again; if it keeps happening, give the details below to an administrator.',
    description: 'Explanation under the crash fallback heading. Says nothing was lost and what to do.',
  },
  retry: { id: 'common.errorBoundary.retry', defaultMessage: 'Try again', description: 'Button that renders the crashed screen again.' },
  details: {
    id: 'common.errorBoundary.details',
    defaultMessage: 'Technical details',
    description: 'Label of the collapsible section with the error message of a crash.',
  },
});

export interface ErrorFallbackProps {
  error: Error;
  reset: () => void;
}

/** What people see when a screen crashes. No mascot: this is an error. */
export function ErrorFallback({ error, reset }: ErrorFallbackProps) {
  const intl = useIntl();
  return (
    <Banner
      tone="danger"
      title={intl.formatMessage(messages.title)}
      action={<Button variant="secondary" onClick={reset}>{intl.formatMessage(messages.retry)}</Button>}
    >
      <p>{intl.formatMessage(messages.hint)}</p>
      <details className="mt-1">
        <summary className="inline-flex min-h-target cursor-pointer items-center rounded-sm underline underline-offset-2">
          {intl.formatMessage(messages.details)}
        </summary>
        <p className="font-mono text-mono wrap-anywhere">{error.message}</p>
      </details>
    </Banner>
  );
}

export interface ErrorBoundaryProps {
  children: ReactNode;
  /** Custom fallback. Defaults to ErrorFallback. */
  fallback?: ComponentType<ErrorFallbackProps>;
  /** Called with the error, for logging. */
  onError?: (error: Error, info: ErrorInfo) => void;
}

interface ErrorBoundaryState {
  error: Error | null;
}

/** Catches render errors below it and shows a recoverable fallback instead of a blank screen. */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    this.props.onError?.(error, info);
  }

  reset = () => this.setState({ error: null });

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;
    const Fallback = this.props.fallback ?? ErrorFallback;
    return <Fallback error={error} reset={this.reset} />;
  }
}
