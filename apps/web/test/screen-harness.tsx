// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Providers } from '@orcon/ui';
import { type QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryHistory, RouterProvider } from '@tanstack/react-router';
import { render, type RenderResult } from '@testing-library/react';
import type { ReactElement } from 'react';

import { emptyLiveEventSource, type LiveEventSource, type LiveHandlers } from '../src/app/live-events/dispatcher';
import { LiveEventsProvider } from '../src/app/live-events/LiveEventsProvider';
import { createQueryClient } from '../src/app/query-client';
import { createAppRouter } from '../src/app/router';
import { appMessages } from '../src/i18n/messages';

export { server } from './server';

export interface HarnessOptions {
  /** The event stream. Defaults to one that never delivers anything. */
  source?: LiveEventSource | undefined;
  handlers?: LiveHandlers | undefined;
  /** A query client, to read the cache the screen filled. Defaults to the app's own defaults. */
  queryClient?: QueryClient | undefined;
}

function wrap(ui: ReactElement, { source = emptyLiveEventSource, handlers, queryClient }: HarnessOptions & { queryClient: QueryClient }) {
  return (
    <Providers messages={appMessages}>
      <QueryClientProvider client={queryClient}>
        <LiveEventsProvider source={source} handlers={handlers}>
          {ui}
        </LiveEventsProvider>
      </QueryClientProvider>
    </Providers>
  );
}

/** Renders a component in the providers of the app: theme, density, locale, queries and live events. */
export function renderInApp(ui: ReactElement, options: HarnessOptions = {}): RenderResult & { queryClient: QueryClient } {
  const queryClient = options.queryClient ?? createQueryClient();
  return { ...render(wrap(ui, { ...options, queryClient })), queryClient };
}

/** Renders the whole app at a path, with its real route tree and an in-memory history. */
export function renderRoute(path: string, options: HarnessOptions = {}) {
  const queryClient = options.queryClient ?? createQueryClient();
  const history = createMemoryHistory({ initialEntries: [path] });
  const router = createAppRouter({ queryClient, history });
  const view = render(wrap(<RouterProvider router={router} />, { ...options, queryClient }));
  return { ...view, router, history, queryClient };
}
