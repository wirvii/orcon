// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { getRouteApi } from '@tanstack/react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { QueryState } from '../../../app/query-state/QueryState';
import type { LoadedDraft } from '../api/draft-source';
import { useSaveDraft } from '../api/mutations';
import { draftQueryOptions, usesLocalDrafts } from '../api/queries';
import { EditorWorkspace } from '../components/EditorWorkspace';
import { editorMessages } from '../components/messages';
import { createEditorStore } from '../store/editor-store';
import { EditorStoreProvider } from '../store/editor-context';

const route = getRouteApi('/flows/$flowId');

/** The store of one open flow lives as long as the screen shows it; a different flow gets a different store. */
function OpenFlow({ flowId, draft }: { flowId: string; draft: LoadedDraft }) {
  const queryClient = useQueryClient();
  const save = useSaveDraft(flowId);
  const [store] = useState(() => createEditorStore({ document: draft.document, revision: draft.revision }));

  useEffect(() => {
    markFirstUsefulScreen('/flows/$flowId');
  }, []);

  const reload = async () => {
    const fresh = await queryClient.query({ ...draftQueryOptions(flowId), staleTime: 0 });
    return { document: fresh.document, revision: fresh.revision };
  };

  return (
    <EditorStoreProvider store={store}>
      <EditorWorkspace save={save} reload={reload} preview={usesLocalDrafts} />
    </EditorStoreProvider>
  );
}

/**
 * The flow editor. It reads the draft once and gives it to a store that holds the working copy;
 * from there the editor changes the store and saves it (orcon-web ST-3, ST-9). Until flows are
 * stored on the server the draft comes from a sample kept in memory and the page says so.
 */
export function EditorScreen() {
  const intl = useIntl();
  const { flowId } = route.useParams();
  const query = useQuery(draftQueryOptions(flowId));
  return (
    <div className="flex min-h-0 flex-1 flex-col" data-editor-screen>
      <QueryState query={query} isEmpty={() => false} empty={{ title: intl.formatMessage(editorMessages.emptyTitle), description: intl.formatMessage(editorMessages.emptyText) }} skeletonRows={4}>
        {(draft) => <OpenFlow key={flowId} flowId={flowId} draft={draft} />}
      </QueryState>
    </div>
  );
}
