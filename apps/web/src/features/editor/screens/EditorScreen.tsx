// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { getRouteApi, Link as RouterLink } from '@tanstack/react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { QueryState } from '../../../app/query-state/QueryState';
import type { LoadedDraft } from '../api/draft-source';
import { useSaveDraft } from '../api/mutations';
import { draftQueryOptions, usesLocalDrafts } from '../api/queries';
import type { EditorNav } from '../components/EditorTopBar';
import { EditorWorkspace } from '../components/EditorWorkspace';
import { editorMessages } from '../components/messages';
import { createEditorStore } from '../store/editor-store';
import { EditorStoreProvider } from '../store/editor-context';

const route = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle/flows/$flowId');
const EDITOR_ROUTE = '/orgs/$orgSlug/projects/$projectHandle/flows/$flowId';

/** The store of one open flow lives as long as the screen shows it; a different flow gets a different store. */
function OpenFlow({ flowId, draft, nav }: { flowId: string; draft: LoadedDraft; nav: EditorNav }) {
  const queryClient = useQueryClient();
  const save = useSaveDraft(flowId);
  const [store] = useState(() => createEditorStore({ document: draft.document, revision: draft.revision }));

  useEffect(() => {
    markFirstUsefulScreen(EDITOR_ROUTE);
  }, []);

  const reload = async () => {
    const fresh = await queryClient.query({ ...draftQueryOptions(flowId), staleTime: 0 });
    return { document: fresh.document, revision: fresh.revision };
  };

  return (
    <EditorStoreProvider store={store}>
      <EditorWorkspace save={save} reload={reload} preview={usesLocalDrafts} flowId={usesLocalDrafts ? undefined : flowId} nav={nav} />
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
  const { orgSlug, projectHandle, flowId } = route.useParams();
  // The project the parent route loaded; the handle stands in while a host does not provide it.
  const projectName = route.useRouteContext({ select: (context) => (context as { project?: { name: string } }).project?.name });
  const query = useQuery(draftQueryOptions(flowId));
  const nav: EditorNav = {
    projectLabel: projectName ?? projectHandle,
    link: (target, { className, children }) =>
      target === 'project' ? (
        <RouterLink to="/orgs/$orgSlug/projects/$projectHandle/flows" params={{ orgSlug, projectHandle }} className={className}>
          {children}
        </RouterLink>
      ) : (
        <RouterLink to="/orgs/$orgSlug/projects/$projectHandle/executions" params={{ orgSlug, projectHandle }} search={{ flowId }} className={className}>
          {children}
        </RouterLink>
      ),
  };
  return (
    <div className="flex min-h-0 flex-1 flex-col" data-editor-screen>
      <QueryState query={query} isEmpty={() => false} empty={{ title: intl.formatMessage(editorMessages.emptyTitle), description: intl.formatMessage(editorMessages.emptyText) }} skeletonRows={4}>
        {(draft) => <OpenFlow key={flowId} flowId={flowId} draft={draft} nav={nav} />}
      </QueryState>
    </div>
  );
}
