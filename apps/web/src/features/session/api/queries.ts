// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';

import { getCurrentUser, listOrganizations } from './operations';

/**
 * Keys of the data read before an organization is active (orcon-web DT-4): they start with the
 * constant `'session'`. Signing out clears the whole cache.
 */
export const sessionKeys = {
  all: ['session'] as const,
  currentUser: () => [...sessionKeys.all, 'current-user'] as const,
  organizations: () => [...sessionKeys.all, 'organizations'] as const,
  organizationList: (slug: string | undefined) => [...sessionKeys.organizations(), { slug: slug ?? null }] as const,
};

export function currentUserQueryOptions() {
  return queryOptions({
    queryKey: sessionKeys.currentUser(),
    queryFn: getCurrentUser,
    staleTime: 5 * 60_000,
  });
}

/**
 * The organizations of the caller, first page. The landing only needs to know whether there are
 * none, one or several, and the picker shows this page; more pages arrive with the list virtualization.
 */
export function organizationsQueryOptions() {
  return queryOptions({
    queryKey: sessionKeys.organizationList(undefined),
    queryFn: async () => (await listOrganizations({ limit: 200 })).items,
  });
}

/** The organization with one slug, if the caller is an active member of it. The list is empty otherwise. */
export function organizationBySlugQueryOptions(slug: string) {
  return queryOptions({
    queryKey: sessionKeys.organizationList(slug),
    queryFn: async () => (await listOrganizations({ slug, limit: 1 })).items,
  });
}
