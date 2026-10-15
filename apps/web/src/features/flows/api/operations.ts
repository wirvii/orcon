// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiRequest } from '../../../app/api/http';

/** A flow as the list reads it (SPEC-016 R-SH-API-02). */
export const flowSchema = z.object({
  id: z.uuid(),
  projectId: z.uuid(),
  name: z.string(),
  activationState: z.enum(['ACTIVE', 'INACTIVE']),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const flowPageSchema = z.object({
  items: z.array(flowSchema),
  nextPageToken: z.string().max(1024).optional(),
});

export type Flow = z.infer<typeof flowSchema>;
export type FlowPage = z.infer<typeof flowPageSchema>;

/** Lists the flows of a project, most recently changed first (the default order of the contract). */
export function listFlows({ projectId, limit = 200 }: { projectId: string; limit?: number | undefined }): Promise<FlowPage> {
  return apiRequest({
    method: 'GET',
    path: `/projects/${encodeURIComponent(projectId)}/flows`,
    query: { limit },
    schema: flowPageSchema,
    source: 'response of GET /projects/{projectId}/flows',
  });
}

/** Creates an inactive flow with an empty draft (SPEC-016 R-SH-FLW-02). */
export function createFlow({ projectId, name }: { projectId: string; name: string }): Promise<Flow> {
  return apiRequest({
    method: 'POST',
    path: `/projects/${encodeURIComponent(projectId)}/flows`,
    body: { name },
    schema: flowSchema,
    source: 'response of POST /projects/{projectId}/flows',
  });
}
