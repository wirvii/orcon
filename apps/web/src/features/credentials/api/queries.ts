// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';

import { credentialStore } from './store';

/**
 * Keys of the credential data (orcon-web DT-3, DT-4): every key starts with the id of the active
 * organization, and the list names its project, so no list of one project shows another's credentials.
 */
export const credentialKeys = {
  all: (orgId: string) => [orgId, 'credentials'] as const,
  list: (orgId: string, projectId: string) => [orgId, 'credentials', 'list', projectId] as const,
};

/** The credentials of the project, names, types and change times only. */
export function credentialsQueryOptions(orgId: string, projectId: string) {
  return queryOptions({
    queryKey: credentialKeys.list(orgId, projectId),
    queryFn: () => credentialStore.list(projectId),
  });
}
