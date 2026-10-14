// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';

import { listProjects } from './operations';

/**
 * Keys of the project data (orcon-web DT-3, DT-4): every key starts with the id of the active
 * organization, so switching organization never shows another organization's projects.
 */
export const projectKeys = {
  all: (orgId: string) => [orgId, 'projects'] as const,
  list: (orgId: string) => [orgId, 'projects', 'list'] as const,
  byHandle: (orgId: string, handle: string) => [orgId, 'projects', 'handle', handle] as const,
};

/** The projects of the organization the person can read, by name. */
export function projectsQueryOptions(orgId: string) {
  return queryOptions({
    queryKey: projectKeys.list(orgId),
    queryFn: async () => (await listProjects({ orgId })).items,
  });
}

/** The project with one handle in the organization, if the person can read it. The list is empty otherwise. */
export function projectByHandleQueryOptions(orgId: string, handle: string) {
  return queryOptions({
    queryKey: projectKeys.byHandle(orgId, handle),
    queryFn: async () => (await listProjects({ orgId, handle, limit: 1 })).items,
  });
}
