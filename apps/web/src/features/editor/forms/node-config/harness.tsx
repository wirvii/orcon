// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createMemoryHistory, createRootRouteWithContext, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router';
import type { ReactNode } from 'react';
import { vi } from 'vitest';

import { project } from '../../../../../test/product-fixtures';
import { renderInApp, server } from '../../../../../test/screen-harness';
import { acme } from '../../../../../test/session-fixtures';
import { credentialApi, modelCredential, slackWebhook, whatsappCredential } from '../../../../app/credentials/api/fixtures';
import { writeStored } from '../../../../app/boundary/storage';
import { createQueryClient } from '../../../../app/query-client';
import type { RouterContext } from '../../../../app/router-context';
import { createLocalFlowTargetSource } from '../../api/local-flow-target-source';
import { flowTargetSource } from '../../api/queries';
import { createNoticeChannel, NoticeContext, type NoticeChannel } from '../../components/notice-channel';
import { exampleFlowDocument } from '../../model/example-flow';
import type { FlowDocument } from '../../model/flow-document';
import { EditorStoreProvider } from '../../store/editor-context';
import { createEditorStore, type EditorStore } from '../../store/editor-store';

export interface Harness {
  store: EditorStore;
  channel: NoticeChannel;
}

/**
 * Renders part of the node details view the way the editor does: a store holding a flow, the notice
 * channel, a query client and a router (the sub-flow widget links to flows). Credentials and flows
 * come from fresh in-memory sources, so no test sees what another created.
 */
export function renderWithStore(ui: ReactNode, document: FlowDocument = exampleFlowDocument(), expressionView: 'chips' | 'raw' = 'raw'): Harness & ReturnType<typeof renderInApp> {
  // The tests of the form type into formulas as text; the ones about chips ask for them.
  writeStored('orcon.editor.expression-view', expressionView);
  // The credentials of the project, as the credential service lists them; a test that needs others puts its own handler first.
  server.use(credentialApi.list([slackWebhook, whatsappCredential, modelCredential]), credentialApi.nodeTypes([]));
  const flows = createLocalFlowTargetSource();
  vi.spyOn(flowTargetSource, 'list').mockImplementation(flows.list);

  const store = createEditorStore({ document, revision: '1' });
  const channel = createNoticeChannel();
  const queryClient = createQueryClient();
  const root = createRootRouteWithContext<RouterContext>()({ component: Outlet });
  const page = createRoute({
    getParentRoute: () => root,
    // The editor sits under its project, so the sub-flow widget finds the organization and project of the address.
    path: '/orgs/$orgSlug/projects/$projectHandle',
    // The project route of the app puts the organization and the project in the context of everything under it.
    beforeLoad: () => ({ organization: acme, project }),
    component: () => (
      <NoticeContext.Provider value={channel}>
        <EditorStoreProvider store={store}>{ui}</EditorStoreProvider>
      </NoticeContext.Provider>
    ),
  });
  const flow = createRoute({ getParentRoute: () => root, path: '/orgs/$orgSlug/projects/$projectHandle/flows/$flowId', component: () => null });
  const router = createRouter({ routeTree: root.addChildren([page, flow]), history: createMemoryHistory({ initialEntries: ['/orgs/acme/projects/ventas'] }), context: { queryClient } });
  const view = renderInApp(<RouterProvider router={router} />, { queryClient });
  return { ...view, store, channel };
}
