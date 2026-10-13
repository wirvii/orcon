// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { QueryClient } from '@tanstack/react-query';
import { createRouter } from '@tanstack/react-router';

import { RouteError } from './errors/RouteError';
import { NotFound } from './shell/NotFound';
import { routeTree } from './routeTree.gen';

/**
 * Builds the router (orcon-web RT-1, RT-5, RT-7). Links preload on intent and the preloaded
 * data is considered stale at once, so a click always shows fresh data.
 */
export function createAppRouter({ queryClient }: { queryClient: QueryClient }) {
  return createRouter({
    routeTree,
    context: { queryClient },
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0,
    defaultErrorComponent: RouteError,
    defaultNotFoundComponent: NotFound,
  });
}

export type AppRouter = ReturnType<typeof createAppRouter>;

// Registers the router type, so every link, navigation and search parameter in the app is type-checked.
declare module '@tanstack/react-router' {
  interface Register {
    router: AppRouter;
  }
}
