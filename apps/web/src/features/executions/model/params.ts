// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

/**
 * The search parameters of the runs list. `flowId` keeps the runs of one flow; a malformed one is simply
 * no filter, so the page falls back to every run (orcon-web RT-3).
 */
export const executionsSearchSchema = z.object({
  flowId: z.uuid().optional().catch(undefined),
});
