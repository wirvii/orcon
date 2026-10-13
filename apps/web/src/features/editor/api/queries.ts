// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';

import { EXAMPLE_FLOW_ID, exampleFlowDocument, PERF_FLOW_ID } from '../model/example-flow';

import type { DraftSource } from './draft-source';
import { createLocalDraftSource, type DraftSeed } from './local-draft-source';

/**
 * The organization of the keys until sign-in provides one (orcon-web DT-4: every key starts with
 * the organization id).
 */
export const LOCAL_ORGANIZATION = 'local';

export const editorKeys = {
  all: (organizationId: string) => [organizationId, 'editor'] as const,
  draft: (organizationId: string, flowId: string) => [organizationId, 'editor', 'draft', flowId] as const,
};

const seeds = new Map<string, DraftSeed>([[EXAMPLE_FLOW_ID, exampleFlowDocument]]);
// The performance build (`vite build --mode perf`) also opens the 500-node reference flow; in the
// production build this condition is false and the module is not part of the bundle.
if (import.meta.env.MODE === 'perf') {
  seeds.set(PERF_FLOW_ID, async () => (await import('./perf-flow')).referenceFlowDocument());
}

/**
 * The one place that chooses where drafts come from. The preview reads and writes in memory
 * (`createLocalDraftSource`); connecting the editor to the server means returning the SDK-backed
 * source here (ED5a part 2). Nothing else in the editor names a source.
 */
export const draftSource: DraftSource = createLocalDraftSource(seeds);

/**
 * The draft of a flow, read once. The editor copies it into its store, and from then on the store
 * holds the working copy: a refetch must never replace what a person is editing (orcon-web ST-9),
 * so this query is never stale and never refetches on focus.
 */
export function draftQueryOptions(flowId: string) {
  return queryOptions({
    queryKey: editorKeys.draft(LOCAL_ORGANIZATION, flowId),
    queryFn: () => draftSource.load(flowId),
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}
