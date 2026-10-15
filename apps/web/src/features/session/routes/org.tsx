// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, notFound, Outlet } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { organizationBySlugQueryOptions } from '../api/queries';
import { orgSlugSchema } from '../../../app/route-params';

/**
 * The organization disposition: the slug in the address must name an organization the person is an
 * active member of. Otherwise the page does not exist for them, which is the same answer as an
 * organization that is not there at all (SPEC-016 R-SH-URL-04).
 */
export const Route = createFileRoute('/_session/orgs/$orgSlug')({
  params: {
    parse: (raw) => {
      const slug = orgSlugSchema.safeParse(raw.orgSlug);
      // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's notFound() is a marker object it catches, not an Error
      if (!slug.success) throw notFound();
      return { orgSlug: slug.data };
    },
    stringify: ({ orgSlug }) => ({ orgSlug }),
  },
  beforeLoad: async ({ context, params }) => {
    const [organization] = await context.queryClient.query(organizationBySlugQueryOptions(params.orgSlug));
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's notFound() is a marker object it catches, not an Error
    if (organization === undefined) throw notFound();
    return { organization };
  },
  errorComponent: RouteError,
  component: () => <Outlet />,
});
