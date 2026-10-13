// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { mutationOptions, useMutation, useQueryClient } from '@tanstack/react-query';

import type { FlowDocument } from '../model/flow-document';

import type { LoadedDraft, SaveResult } from './draft-source';
import { draftSource, editorKeys, LOCAL_ORGANIZATION } from './queries';

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
      }
    },
  });
  return mutation.mutateAsync;
}
