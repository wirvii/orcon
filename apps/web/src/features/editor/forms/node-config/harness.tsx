// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createMemoryHistory, createRootRouteWithContext, createRoute, createRouter, Outlet, RouterProvider } from '@tanstack/react-router';
import type { ReactNode } from 'react';
import { vi } from 'vitest';

import { renderInApp } from '../../../../../test/screen-harness';
import { createQueryClient } from '../../../../app/query-client';
import type { RouterContext } from '../../../../app/router-context';
import { createLocalCredentialSource } from '../../api/local-credential-source';
import { createLocalFlowTargetSource } from '../../api/local-flow-target-source';
import { credentialSource, flowTargetSource } from '../../api/queries';
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
export function renderWithStore(ui: ReactNode, document: FlowDocument = exampleFlowDocument()): Harness & ReturnType<typeof renderInApp> {
  const fresh = createLocalCredentialSource();
  vi.spyOn(credentialSource, 'list').mockImplementation(fresh.list);
  vi.spyOn(credentialSource, 'create').mockImplementation(fresh.create);
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
