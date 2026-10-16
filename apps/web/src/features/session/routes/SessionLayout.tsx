// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { problemStatus } from '../../../app/errors/to-problem-details';
import { currentUserQueryOptions } from '../../../app/session/current-user';
import { SessionFrame } from '../screens/SessionFrame';

/**
 * Every signed-in page is below this layout. It reads the current user before any of them renders;
 * without a valid session the person is sent to sign-in and comes back to this address afterwards
 * (orcon-web RT-10, SPEC-016 R-SH-URL-02).
 */
function SessionLayout() {
  return (
    <SessionFrame>
      <Outlet />
    </SessionFrame>
  );
}

export const Route = createFileRoute('/_session')({
  beforeLoad: async ({ context, location }) => {
    try {
      await context.queryClient.query(currentUserQueryOptions());
    } catch (error) {
      // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
      if (problemStatus(error) === 401) throw redirect({ to: '/login', search: { redirect: location.href } });
      throw error;
    }
  },
  errorComponent: RouteError,
  component: SessionLayout,
});
