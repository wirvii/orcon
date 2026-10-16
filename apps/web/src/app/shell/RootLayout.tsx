// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Toaster } from '@orcon/ui';
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router';

import { RouteError } from '../errors/RouteError';
import type { RouterContext } from '../router-context';
import { NotFound } from './NotFound';

/**
 * The root of every page. It adds no frame of its own: the signed-in pages wrap themselves in
 * `AppFrame`, the sign-in page draws its own layout, so each frame appears exactly once.
 */
function RootLayout() {
  return (
    <>
      <Outlet />
      {/* The one place short confirmations appear, such as the undo notice after deleting nodes on the canvas. */}
      <Toaster />
    </>
  );
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  errorComponent: RouteError,
  notFoundComponent: NotFound,
});
