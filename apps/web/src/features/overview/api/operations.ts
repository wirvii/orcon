// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiRequest } from '../../../app/api/http';

/** The figures of the home page (`getOrgActivitySummary`, SPEC-019 UXB2). Every count covers the projects the person can read. */
export const activitySummarySchema = z.object({
  activeFlowCount: z.number().int().nonnegative(),
  flowCount: z.number().int().nonnegative(),
  projectCount: z.number().int().nonnegative(),
  executionsLast24h: z.object({
    total: z.number().int().nonnegative(),
    failed: z.number().int().nonnegative(),
    succeeded: z.number().int().nonnegative(),
    uncertain: z.number().int().nonnegative(),
    /** The oldest hour first; the last item is the current hour. */
    byHour: z.array(z.number().int().nonnegative()),
  }),
  timeSaved: z.object({
    windowDays: z.number().int().positive(),
    succeededExecutions: z.number().int().nonnegative(),
    minutesPerExecution: z.number().int().nonnegative(),
    minutes: z.number().int().nonnegative(),
  }),
});

export type ActivitySummary = z.infer<typeof activitySummarySchema>;

export function getOrgActivitySummary(orgId: string): Promise<ActivitySummary> {
  return apiRequest({
    method: 'GET',
    path: `/orgs/${encodeURIComponent(orgId)}/activity-summary`,
    schema: activitySummarySchema,
    source: 'response of GET /orgs/{orgId}/activity-summary',
  });
}
