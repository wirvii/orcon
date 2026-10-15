// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { index, layout, physical, route, rootRoute } from '@tanstack/virtual-file-routes';

/**
 * The route tree is built from virtual file routes: the root layout lives in `app/shell/`, and every
 * feature mounts its `routes/` folder next to its screens (orcon-web RT-1). The session area wraps
 * the signed-in pages in one layout that checks the session before any of them renders (RT-10).
 */
export const routes = rootRoute('shell/RootLayout.tsx', [
  route('/login', '../features/session/routes/login.tsx'),
  layout('session', '../features/session/routes/SessionLayout.tsx', [
    index('../features/session/routes/landing.tsx'),
    route('/orgs', '../features/session/routes/orgs.tsx'),
    route('/orgs/$orgSlug', '../features/session/routes/org.tsx', [
      index('../features/projects/routes/org-index.tsx'),
      route('/projects', '../features/projects/routes/projects.tsx'),
      route('/projects/$projectHandle', '../features/projects/routes/project.tsx', [
        index('../features/projects/routes/project-index.tsx'),
        route('/flows', '../features/flows/routes/flows.tsx'),
        route('/credentials', '../features/credentials/routes/credentials.tsx'),
        route('/flows/$flowId', '../features/editor/routes/$flowId.tsx'),
        route('/executions', '../features/executions/routes/executions.tsx'),
        route('/executions/$executionId', '../features/executions/routes/execution.tsx'),
      ]),
    ]),
    // The address the editor had before flows lived under a project. It only forwards (see the file).
    route('/flows/$flowId', '../features/editor/routes/legacy-flow.tsx'),
  ]),
  // Public, but framed like the signed-in pages when a session exists (see the layout file).
  route('/settings', '../features/session/routes/SettingsLayout.tsx', [physical('', '../features/settings/routes')]),
]);
