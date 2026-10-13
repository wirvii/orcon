// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import './styles.css';

import { ErrorBoundary, Providers } from '@orcon/ui';
import { QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { emptyLiveEventSource } from './app/live-events/dispatcher';
import { LiveEventsProvider } from './app/live-events/LiveEventsProvider';
import { createQueryClient } from './app/query-client';
import { createAppRouter } from './app/router';
import { appMessages } from './i18n/messages';

const root = document.getElementById('root');
if (!root) throw new Error('The page has no #root element to mount the app in.');

const queryClient = createQueryClient();
const router = createAppRouter({ queryClient });

createRoot(root).render(
  <StrictMode>
    <Providers messages={appMessages}>
      <ErrorBoundary>
        <QueryClientProvider client={queryClient}>
          {/* The real stream comes from @orcon/sdk (SPEC-005); until then there is nothing to listen to. */}
          <LiveEventsProvider source={emptyLiveEventSource}>
            <RouterProvider router={router} />
          </LiveEventsProvider>
        </QueryClientProvider>
      </ErrorBoundary>
    </Providers>
  </StrictMode>,
);
