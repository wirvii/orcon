// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, notFound, Outlet } from '@tanstack/react-router';

import { RouteError } from '../../../app/errors/RouteError';
import { projectHandleSchema } from '../../../app/route-params';
import { projectByHandleQueryOptions } from '../api/queries';

/**
 * The project disposition: the handle in the address must name a project the person can read in
 * this organization. Otherwise the page does not exist for them (SPEC-016 R-SH-URL-04).
 */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects/$projectHandle')({
  params: {
    parse: (raw) => {
      const handle = projectHandleSchema.safeParse(raw.projectHandle);
      // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's notFound() is a marker object it catches, not an Error
      if (!handle.success) throw notFound();
      return { projectHandle: handle.data };
    },
    stringify: ({ projectHandle }) => ({ projectHandle }),
  },
  beforeLoad: async ({ context, params }) => {
    const [project] = await context.queryClient.query(projectByHandleQueryOptions(context.organization.id, params.projectHandle));
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's notFound() is a marker object it catches, not an Error
    if (project === undefined) throw notFound();
    return { project };
  },
  errorComponent: RouteError,
  component: () => <Outlet />,
});
