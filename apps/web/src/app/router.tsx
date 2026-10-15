// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { QueryClient } from '@tanstack/react-query';
import { createRouter, type RouterHistory } from '@tanstack/react-router';

import { RouteError } from './errors/RouteError';
import { onUnauthenticated } from './query-client';
import { NotFound } from './shell/NotFound';
import { routeTree } from './routeTree.gen';

/**
 * Builds the router (orcon-web RT-1, RT-5, RT-7). Links preload on intent and the preloaded
 * data is considered stale at once, so a click always shows fresh data. Tests pass an in-memory
 * `history`; the app uses the browser's.
 */
export function createAppRouter({ queryClient, history }: { queryClient: QueryClient; history?: RouterHistory | undefined }) {
  const router = createRouter({
    routeTree,
    ...(history ? { history } : {}),
    context: { queryClient },
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0,
    defaultErrorComponent: RouteError,
    defaultNotFoundComponent: NotFound,
  });
  // A 401 on any request of the page ends the session: the person is sent to sign-in with the address they
  // were on, and everything read for them is dropped (orcon-web DT-4).
  onUnauthenticated(queryClient, () => {
    const here = router.state.location;
    if (here.pathname === '/login') return;
    void router.navigate({ to: '/login', search: { redirect: here.href } }).finally(() => {
      queryClient.clear();
    });
  });
  return router;
}

export type AppRouter = ReturnType<typeof createAppRouter>;

// Registers the router type, so every link, navigation and search parameter in the app is type-checked.
declare module '@tanstack/react-router' {
  interface Register {
    router: AppRouter;
  }
}
