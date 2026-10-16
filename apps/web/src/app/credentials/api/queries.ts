// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';

import { getCredential, listCredentials, listNodeTypeIcons } from './operations';

/**
 * Keys of the credential data (orcon-web DT-3, DT-4): every key starts with the id of the active
 * organization, and the list names its project, so no list of one project shows another's credentials.
 * The flow editor keeps its own list under a key that also names credentials; invalidating by that word reaches both.
 */
export const credentialKeys = {
  all: (orgId: string) => [orgId, 'credentials'] as const,
  list: (orgId: string, projectId: string) => [orgId, 'credentials', 'list', projectId] as const,
  detail: (orgId: string, credentialId: string) => [orgId, 'credentials', 'detail', credentialId] as const,
  icons: (orgId: string, projectId: string) => [orgId, 'credentials', 'icons', projectId] as const,
};

/** The credentials of the project: names, services, allowed hosts and change times. No secret value. */
export function credentialsQueryOptions(orgId: string, projectId: string) {
  return queryOptions({
    queryKey: credentialKeys.list(orgId, projectId),
    queryFn: () => listCredentials(projectId),
  });
}

/** One credential with the entity tag a change must send. Always read afresh: the tag is only good while nobody else changed it. */
export function credentialQueryOptions(orgId: string, credentialId: string) {
  return queryOptions({
    queryKey: credentialKeys.detail(orgId, credentialId),
    queryFn: () => getCredential(credentialId),
    gcTime: 0,
    staleTime: 0,
  });
}

/** The brand icons of the services, from the node types of the project. A failure leaves the services with the plug glyph. */
export function serviceIconsQueryOptions(orgId: string, projectId: string) {
  return queryOptions({
    queryKey: credentialKeys.icons(orgId, projectId),
    queryFn: () => listNodeTypeIcons(projectId),
    retry: false,
    staleTime: 5 * 60_000,
  });
}
