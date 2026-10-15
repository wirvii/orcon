// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, redirect } from '@tanstack/react-router';

/** A project's own address opens its flows (SPEC-016 §3.7). */
export const Route = createFileRoute('/_session/orgs/$orgSlug/projects/$projectHandle/')({
  beforeLoad: ({ params }) => {
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
    throw redirect({ to: '/orgs/$orgSlug/projects/$projectHandle/flows', params: { orgSlug: params.orgSlug, projectHandle: params.projectHandle } });
  },
});
