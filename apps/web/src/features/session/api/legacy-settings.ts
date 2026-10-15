// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { QueryClient } from '@tanstack/react-query';
import { redirect } from '@tanstack/react-router';

import { problemStatus } from '../../../app/errors/to-problem-details';
import { rememberedOrganization } from './last-org';
import { currentUserQueryOptions } from '../../../app/session/current-user';
import { organizationsQueryOptions } from './queries';

/**
 * The settings pages used to live outside the organizations, at `/settings/appearance`, and were public. They live
 * inside the organization now (SPEC-019 Q4); this sends an old link there so no saved link breaks. A person with a
 * session goes to the same section of the organization they were in, or of the only one they belong to, or to the
 * picker when that is not clear; without a session the person goes to sign-in, where language and theme can be chosen.
 * Always throws: it is the whole body of the route.
 */
export async function redirectLegacySettings(queryClient: QueryClient, section: 'appearance' | 'profile'): Promise<never> {
  try {
    await queryClient.query(currentUserQueryOptions());
  } catch (error) {
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
    if (problemStatus(error) === 401) throw redirect({ to: '/login' });
    throw error;
  }
  const organizations = await queryClient.query(organizationsQueryOptions());
  const remembered = rememberedOrganization();
  const slug = organizations.find((organization) => organization.slug === remembered)?.slug ?? (organizations.length === 1 ? organizations[0]?.slug : undefined);
  // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
  if (slug === undefined) throw redirect({ to: '/orgs' });
  // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
  throw redirect({ to: section === 'appearance' ? '/orgs/$orgSlug/settings/appearance' : '/orgs/$orgSlug/settings/profile', params: { orgSlug: slug } });
}
