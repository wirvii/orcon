// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, redirect } from '@tanstack/react-router';

/** An organization's own address opens its projects (SPEC-016 R-SH-URL-03: the index is the list). */
export const Route = createFileRoute('/_session/orgs/$orgSlug/')({
  beforeLoad: ({ params }) => {
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
    throw redirect({ to: '/orgs/$orgSlug/projects', params: { orgSlug: params.orgSlug } });
  },
});
