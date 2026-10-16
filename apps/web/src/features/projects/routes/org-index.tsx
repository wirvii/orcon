// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, redirect } from '@tanstack/react-router';

/** An organization's own address opens its home page (SPEC-019 Q1, which replaces SPEC-016 R-SH-URL-03). */
export const Route = createFileRoute('/_session/orgs/$orgSlug/')({
  beforeLoad: ({ params }) => {
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
    throw redirect({ to: '/orgs/$orgSlug/overview', params: { orgSlug: params.orgSlug } });
  },
});
