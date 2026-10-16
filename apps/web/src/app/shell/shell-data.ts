// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { apiRequest } from '../api/http';

/**
 * What the frame of the signed-in pages reads for every screen: the projects the person can read (the sidebar) and,
 * for the home page, the flows of each. It lives in `app/` because several areas show it and areas do not import
 * each other (orcon-web AR-6). The keys start with the id of the active organization and sit under the keys of the
 * project and flow areas (`[orgId, 'projects']`, `[orgId, 'flows']`), so creating a project or a flow refreshes them;
 * they end in `shell` because the shape they hold is this narrower one, not the area's own.
 */

const shellProjectSchema = z.object({
  id: z.uuid(),
  name: z.string(),
  handle: z.string(),
  /** How many flows the project has. The server sends it once the contract has it (SPEC-019 UXB1); until then it is absent. */
  flowCount: z.number().int().nonnegative().optional(),
});

const shellProjectPageSchema = z.object({ items: z.array(shellProjectSchema) });

const shellFlowSchema = z.object({
  id: z.uuid(),
  projectId: z.uuid(),
  name: z.string(),
  activationState: z.enum(['ACTIVE', 'INACTIVE']),
  updatedAt: z.string(),
});

const shellFlowPageSchema = z.object({ items: z.array(shellFlowSchema) });

export type ShellProject = z.infer<typeof shellProjectSchema>;
export type ShellFlow = z.infer<typeof shellFlowSchema>;

export const shellKeys = {
  projects: (orgId: string) => [orgId, 'projects', 'shell'] as const,
  flows: (orgId: string, projectId: string) => [orgId, 'flows', 'shell', projectId] as const,
};

/** The projects of the organization the person can read, by name. */
export function shellProjectsQueryOptions(orgId: string) {
  return queryOptions({
    queryKey: shellKeys.projects(orgId),
    queryFn: async () =>
      (
        await apiRequest({
          method: 'GET',
          path: `/orgs/${encodeURIComponent(orgId)}/projects`,
          query: { limit: 200 },
          schema: shellProjectPageSchema,
          source: 'response of GET /orgs/{orgId}/projects',
        })
      ).items,
  });
}

/** The flows of one project, most recently changed first. */
export function shellFlowsQueryOptions(orgId: string, projectId: string) {
  return queryOptions({
    queryKey: shellKeys.flows(orgId, projectId),
    queryFn: async () =>
      (
        await apiRequest({
          method: 'GET',
          path: `/projects/${encodeURIComponent(projectId)}/flows`,
          query: { limit: 200 },
          schema: shellFlowPageSchema,
          source: 'response of GET /projects/{projectId}/flows',
        })
      ).items,
  });
}
