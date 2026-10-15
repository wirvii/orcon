// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, Outlet } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { PageShell } from '../../../app/shell/PageShell';
import { getCurrentUser } from '../api/operations';
import { currentUserQueryOptions } from '../api/queries';
import { SessionShell } from '../screens/SessionShell';

/**
 * The frame of the settings pages. They are public (a person picks theme and language before signing in,
 * SPEC-016 C-8), but a person who is signed in sees them inside the same frame as every other page:
 * navigation and account menu. Without a session the plain frame is used.
 */
function SettingsLayout() {
  const { signedIn } = Route.useRouteContext();
  const Shell = signedIn ? SessionShell : PageShell;
  return (
    <Shell>
      <Outlet />
    </Shell>
  );
}

// The only server read is whether a session exists. It is a plain request, not a query: a 401 here is an answer
// ("not signed in"), not a lost session, so it must not send the person to sign-in (orcon-web RT-10).
export const Route = createFileRoute('/settings')({
  beforeLoad: async ({ context }) => {
    try {
      const user = await getCurrentUser();
      context.queryClient.setQueryData(currentUserQueryOptions().queryKey, user);
      return { signedIn: true };
    } catch {
      return { signedIn: false };
    }
  },
  errorComponent: RouteError,
  component: SettingsLayout,
});
