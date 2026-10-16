// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, redirect } from '@tanstack/react-router';

/** The address of Settings alone opens the first section, the person's profile. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/settings/')({
  beforeLoad: ({ params }) => {
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
    throw redirect({ to: '/orgs/$orgSlug/settings/profile', params: { orgSlug: params.orgSlug } });
  },
});
