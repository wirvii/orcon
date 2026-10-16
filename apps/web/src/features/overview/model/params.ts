// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

/**
 * What the address of the home page keeps: the tab, the search, the state filter and the order, so a view can be
 * shared and survives a reload. A malformed value is no value (the page falls back to its default).
 */
export const overviewSearchSchema = z.object({
  tab: z.enum(['flows', 'credentials', 'executions']).optional().catch(undefined),
  q: z.string().max(200).optional().catch(undefined),
  state: z.enum(['ACTIVE', 'INACTIVE']).optional().catch(undefined),
  sort: z.enum(['-updatedAt', '-lastExecutionAt']).optional().catch(undefined),
});

export type OverviewSearch = z.infer<typeof overviewSearchSchema>;
export type OverviewTab = NonNullable<OverviewSearch['tab']>;
