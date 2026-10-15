// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, redirect } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { organizationsQueryOptions } from '../api/queries';
import { landingFor } from '../model/landing';
import { landingSearchSchema } from '../model/params';
import { NoOrganizationScreen } from '../screens/NoOrganizationScreen';

/**
 * The start address of a signed-in person. It sends them where their organizations say (one
 * organization: its projects; several: the picker) or, with no organization, shows the explanation.
 */
export const Route = createFileRoute('/_session/')({
  validateSearch: landingSearchSchema,
  beforeLoad: async ({ context, search }) => {
    const organizations = await context.queryClient.query(organizationsQueryOptions());
    const landing = landingFor(organizations, search.redirect);
    // redirect() returns a marker object the router catches, not an Error, so the throws below are correct.
    switch (landing.kind) {
      case 'redirect':
        // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
        throw redirect({ href: landing.href });
      case 'organization':
        // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
        throw redirect({ to: '/orgs/$orgSlug', params: { orgSlug: landing.slug } });
      case 'organizations':
        // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
        throw redirect({ to: '/orgs' });
      case 'none':
        return;
    }
  },
  errorComponent: RouteError,
  component: NoOrganizationScreen,
});
