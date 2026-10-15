// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

/** The four ways to start a flow (SPEC-019 UX4): the chooser opens on one of them. */
export const createModes = ['blank', 'template', 'import', 'ai'] as const;
export type CreateMode = (typeof createModes)[number];

/**
 * What the address of the flows of a project keeps. `create` opens the chooser on arrival, so the key N, the sidebar
 * and the home page lead straight to it: `true` opens it on the first option, a mode opens it on that option. The
 * search, the state filter and the order keep a view shareable. A malformed value is no value.
 */
export const flowsSearchSchema = z.object({
  create: z.union([z.boolean(), z.enum(createModes)]).optional().catch(undefined),
  q: z.string().max(200).optional().catch(undefined),
  state: z.enum(['ACTIVE', 'INACTIVE']).optional().catch(undefined),
  sort: z.enum(['-updatedAt', '-lastExecutionAt']).optional().catch(undefined),
});

export type FlowsSearch = z.infer<typeof flowsSearchSchema>;
