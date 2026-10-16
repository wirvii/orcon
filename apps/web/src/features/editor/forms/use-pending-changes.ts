// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query';
import { useDeferredValue, useMemo } from 'react';

import { flowQueryOptions, versionDocumentQueryOptions } from '../api/queries';
import { pendingChanges } from '../model/pending-changes';
import { useEditorStore } from '../store/editor-context';

/**
 * The changes the working copy has that the published version does not, for the Publish button (SPEC-019
 * R-UX-EDT-02). Without a published version, or while its document is being read, there is no number.
 */
export function usePendingChanges(flowId: string | undefined): number | undefined {
  const flow = useQuery({ ...flowQueryOptions(flowId ?? ''), enabled: flowId !== undefined });
  const version = flow.data?.flow.activeVersion ?? undefined;
  const published = useQuery({ ...versionDocumentQueryOptions(flowId ?? '', version ?? 1), enabled: flowId !== undefined && version !== undefined });
  const document = useEditorStore((state) => state.document);
  const unsaved = useEditorStore((state) => state.version !== state.savedVersion);
  // The server says the saved draft is the published version and nothing is waiting to be saved: nothing to count.
  const identical = flow.data?.flow.hasUnpublishedChanges === false && !unsaved;
  // The count is worked out after the screen has answered the edit, so typing and dragging never wait for it.
  const deferred = useDeferredValue(document);
  return useMemo(() => (published.data === undefined || identical ? undefined : pendingChanges(deferred, published.data)), [deferred, published.data, identical]);
}
