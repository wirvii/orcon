// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { mutationOptions, type QueryClient, useMutation, useQueryClient } from '@tanstack/react-query';

import type { FlowDocument } from '../model/flow-document';

import { type FlowAnswer, publishFlow, setFlowActivation } from './flow-operations';
import type { LoadedDraft, SaveResult } from './draft-source';
import { draftQueryOptions, draftSource, editorKeys, LOCAL_ORGANIZATION } from './queries';

export interface SaveDraftInput {
  document: FlowDocument;
  /** The revision the working copy started from (`If-Match`). */
  ifMatch: string;
}

/**
 * Saves the working copy of the draft. It is never retried by Query: the autosave decides when to
 * try again (it waits longer after each failure), and a conflict is an answer, not an error. A save
 * is not an optimistic update (orcon-web DT-17): the screen shows "Saved" only after the answer.
 */
export function saveDraftMutationOptions(flowId: string) {
  return mutationOptions<SaveResult, Error, SaveDraftInput>({
    mutationKey: [...editorKeys.draft(LOCAL_ORGANIZATION, flowId), 'save'],
    mutationFn: ({ document, ifMatch }) => draftSource.save(flowId, document, ifMatch),
    retry: false,
  });
}

/**
 * The save function of one flow. After a save the cached draft is the one just saved, so opening
 * the flow again starts from it and not from the copy read the first time.
 */
export function useSaveDraft(flowId: string): (input: SaveDraftInput) => Promise<SaveResult> {
  const queryClient = useQueryClient();
  const mutation = useMutation({
    ...saveDraftMutationOptions(flowId),
    onSuccess: (result, input) => {
      if (result.kind === 'saved') {
        queryClient.setQueryData<LoadedDraft>(editorKeys.draft(LOCAL_ORGANIZATION, flowId), { document: input.document, revision: result.revision });
        // The flow now has unpublished changes the server knows about: the publish controls read it again.
        void queryClient.invalidateQueries({ queryKey: editorKeys.flow(LOCAL_ORGANIZATION, flowId) });
      }
    },
  });
  return mutation.mutateAsync;
}

/**
 * Publishes the draft with an optional release note (SPEC-014 ED10a). The publication is based on the
 * entity tag of the DRAFT, the revision the editor last saved or read (never the flow's tag); a draft that
 * moved on is a 412 and nothing is published. A publication is not retried: the person decides to publish
 * again. After every answer, success or refusal, the flow and the draft are read again.
 */
export function usePublishFlow(flowId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationKey: [...editorKeys.flow(LOCAL_ORGANIZATION, flowId), 'publish'],
    mutationFn: async (note: string | undefined) => {
      const draft = queryClient.getQueryData<LoadedDraft>(editorKeys.draft(LOCAL_ORGANIZATION, flowId)) ?? (await queryClient.query(draftQueryOptions(flowId)));
      return publishFlow(flowId, note, draft.revision);
    },
    retry: false,
    onSettled: () => refreshFlowAndDraft(queryClient, flowId),
  });
}

/**
 * Activates or deactivates a flow against the entity tag of the FLOW it was read at (SPEC-014 ED10a). The
 * answer carries the new state and its tag, so the next change is based on it; a stale tag is a conflict
 * and nothing changes. After every answer the flow and the draft are read again.
 */
export function useSetFlowActivation(flowId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationKey: [...editorKeys.flow(LOCAL_ORGANIZATION, flowId), 'activation'],
    mutationFn: ({ active, etag }: { active: boolean; etag: string }) => setFlowActivation(flowId, active, etag),
    retry: false,
    onSuccess: (answer: FlowAnswer) => {
      queryClient.setQueryData<FlowAnswer>(editorKeys.flow(LOCAL_ORGANIZATION, flowId), answer);
    },
    onSettled: () => refreshFlowAndDraft(queryClient, flowId),
  });
}

/** Reads the flow and its draft again: both entity tags may have moved. */
function refreshFlowAndDraft(queryClient: QueryClient, flowId: string): Promise<unknown> {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: editorKeys.flow(LOCAL_ORGANIZATION, flowId) }),
    queryClient.invalidateQueries({ queryKey: editorKeys.draft(LOCAL_ORGANIZATION, flowId), exact: true }),
  ]);
}
