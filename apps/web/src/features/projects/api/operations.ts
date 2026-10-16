// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiRequest } from '../../../app/api/http';

/** A project as the screens read it (SPEC-016 §3.1; the contract may add fields). */
export const projectSchema = z.object({
  id: z.uuid(),
  orgId: z.uuid(),
  name: z.string(),
  handle: z.string(),
  environment: z.string().nullish(),
  protected: z.boolean(),
  /** How many flows the project has, in lists of projects (SPEC-019 UXB1); absent from older servers. */
  flowCount: z.number().int().nonnegative().optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const projectPageSchema = z.object({
  items: z.array(projectSchema),
  nextPageToken: z.string().max(1024).optional(),
});

export type Project = z.infer<typeof projectSchema>;
export type ProjectPage = z.infer<typeof projectPageSchema>;

/** Lists the projects of an organization the person can read. `handle` names one project exactly. */
export function listProjects({ orgId, handle, limit = 200 }: { orgId: string; handle?: string | undefined; limit?: number | undefined }): Promise<ProjectPage> {
  return apiRequest({
    method: 'GET',
    path: `/orgs/${encodeURIComponent(orgId)}/projects`,
    query: { handle, limit },
    schema: projectPageSchema,
    source: 'response of GET /orgs/{orgId}/projects',
  });
}

export interface NewProject {
  orgId: string;
  name: string;
  environment?: string | undefined;
}

/** Creates a project; the creator becomes its administrator (SPEC-016 Q3). */
export function createProject({ orgId, name, environment }: NewProject): Promise<Project> {
  return apiRequest({
    method: 'POST',
    path: `/orgs/${encodeURIComponent(orgId)}/projects`,
    body: environment === undefined || environment === '' ? { name } : { name, environment },
    schema: projectSchema,
    source: 'response of POST /orgs/{orgId}/projects',
  });
}
