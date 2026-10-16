// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

/**
 * What the address of the list of runs keeps: which runs to show, the date range, the flow, the project and the search.
 * A view can be shared and survives a reload. A malformed value is simply no filter, so the page falls back to every
 * run (orcon-web RT-3). `show` is `failed` or `review` ("Needs review", the runs with an operation of unknown
 * outcome); no `show` is every run.
 */
export const executionsSearchSchema = z.object({
  show: z.enum(['failed', 'review']).optional().catch(undefined),
  range: z.enum(['24h', '7d', '30d']).optional().catch(undefined),
  flowId: z.uuid().optional().catch(undefined),
  projectId: z.uuid().optional().catch(undefined),
  /** The identifier of one run. It is kept in the address only when it is a whole identifier. */
  q: z.uuid().optional().catch(undefined),
});

export type ExecutionsSearch = z.infer<typeof executionsSearchSchema>;
export type ExecutionShow = NonNullable<ExecutionsSearch['show']>;
export type ExecutionRange = NonNullable<ExecutionsSearch['range']>;

const HOUR = 3_600_000;
const SPAN: Record<ExecutionRange, number> = { '24h': 24 * HOUR, '7d': 7 * 24 * HOUR, '30d': 30 * 24 * HOUR };

/** The instant a date range starts at, counted back from `now`. */
export function rangeStart(range: ExecutionRange, now: Date): string {
  return new Date(now.getTime() - SPAN[range]).toISOString();
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * What the search box holds, read as an identifier. The search takes a whole run identifier only: looking inside the
 * messages of the runs needs a security decision first (SPEC-019 Q9), so anything else is `invalid` and filters nothing.
 */
export function readSearchText(text: string): { kind: 'empty' } | { kind: 'id'; id: string } | { kind: 'invalid' } {
  const trimmed = text.trim();
  if (trimmed === '') return { kind: 'empty' };
  return UUID.test(trimmed) ? { kind: 'id', id: trimmed.toLowerCase() } : { kind: 'invalid' };
}
