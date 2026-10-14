// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { queryOptions } from '@tanstack/react-query';

import { EXAMPLE_FLOW_ID, exampleFlowDocument, PERF_FLOW_ID, SAMPLE_SUBFLOWS, subflowDocument } from '../model/example-flow';

import type { CredentialSource } from './credential-source';
import type { DraftSource } from './draft-source';
import type { FlowTargetSource } from './flow-target-source';
import { createLocalCredentialSource } from './local-credential-source';
import { createLocalDraftSource, type DraftSeed } from './local-draft-source';
import { createServerDraftSource } from './server-draft-source';
import { createLocalFlowTargetSource } from './local-flow-target-source';

/**
 * The organization of the keys until sign-in provides one (orcon-web DT-4: every key starts with
 * the organization id).
 */
export const LOCAL_ORGANIZATION = 'local';

export const editorKeys = {
  all: (organizationId: string) => [organizationId, 'editor'] as const,
  draft: (organizationId: string, flowId: string) => [organizationId, 'editor', 'draft', flowId] as const,
  credentials: (organizationId: string) => [organizationId, 'editor', 'credentials'] as const,
  flowTargets: (organizationId: string) => [organizationId, 'editor', 'flow-targets'] as const,
};

const seeds = new Map<string, DraftSeed>([
  [EXAMPLE_FLOW_ID, exampleFlowDocument],
  ...SAMPLE_SUBFLOWS.map((flow): [string, DraftSeed] => [flow.id, () => subflowDocument(flow.id, flow.name)]),
]);
// The performance build (`vite build --mode perf`) also opens the 500-node reference flow; in the
// production build this condition is false and the module is not part of the bundle.
if (import.meta.env.MODE === 'perf') {
  seeds.set(PERF_FLOW_ID, async () => (await import('./perf-flow')).referenceFlowDocument());
}

/**
 * The one place that chooses where drafts come from. The application reads and saves drafts through
 * the API (`createServerDraftSource`). The performance build has no server: it keeps the drafts in
 * memory, seeded with the reference flow, so the measurements do not depend on the network.
 */
export const usesLocalDrafts = import.meta.env.MODE === 'perf';

export const draftSource: DraftSource = usesLocalDrafts ? createLocalDraftSource(seeds) : createServerDraftSource();

/** Where the credential widget reads and creates credentials: the same seam as `draftSource` (ED5a part 2 swaps it). */
export const credentialSource: CredentialSource = createLocalCredentialSource();

/** Where the sub-flow widget reads the flows a node may call. */
export const flowTargetSource: FlowTargetSource = createLocalFlowTargetSource();

/** The credentials the person can use in this project (`credential:use`), names and types only. */
export function credentialsQueryOptions() {
  return queryOptions({
    queryKey: editorKeys.credentials(LOCAL_ORGANIZATION),
    queryFn: () => credentialSource.list(),
    staleTime: 60_000,
  });
}

/** The flows a Call sub-flow node may target. */
export function flowTargetsQueryOptions() {
  return queryOptions({
    queryKey: editorKeys.flowTargets(LOCAL_ORGANIZATION),
    queryFn: () => flowTargetSource.list(),
    staleTime: 60_000,
  });
}

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
