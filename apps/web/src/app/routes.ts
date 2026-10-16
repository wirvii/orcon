// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { index, layout, route, rootRoute } from '@tanstack/virtual-file-routes';

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
      route('/overview', '../features/overview/routes/overview.tsx'),
      route('/projects', '../features/projects/routes/projects.tsx'),
      route('/templates', '../features/templates/routes/templates.tsx'),
      route('/credentials', '../features/credentials/routes/org-credentials.tsx'),
      route('/executions', '../features/executions/routes/org-executions.tsx', [
        index('../features/executions/routes/org-executions-index.tsx'),
        route('/$executionId', '../features/executions/routes/org-execution.tsx'),
      ]),
      // Settings live inside the organization, so the frame (sidebar and account menu) stays (SPEC-019 Q4).
      route('/settings', '../features/settings/routes/settings.tsx', [
        index('../features/settings/routes/settings-index.tsx'),
        route('/profile', '../features/settings/routes/profile.tsx'),
        route('/security', '../features/settings/routes/security.tsx'),
        route('/appearance', '../features/settings/routes/appearance.tsx'),
        route('/organization', '../features/settings/routes/organization.tsx'),
        route('/members', '../features/settings/routes/members.tsx'),
        route('/api-keys', '../features/settings/routes/api-keys.tsx'),
      ]),
      route('/projects/$projectHandle', '../features/projects/routes/project.tsx', [
        index('../features/projects/routes/project-index.tsx'),
        route('/flows', '../features/flows/routes/flows.tsx'),
        route('/credentials', '../features/credentials/routes/credentials.tsx'),
        route('/flows/$flowId', '../features/editor/routes/$flowId.tsx'),
        route('/executions', '../features/executions/routes/executions.tsx'),
        route('/executions/$executionId', '../features/executions/routes/execution.tsx'),
        route('/settings', '../features/projects/routes/project-settings.tsx'),
      ]),
    ]),
    // The address the editor had before flows lived under a project. It only forwards (see the file).
    route('/flows/$flowId', '../features/editor/routes/legacy-flow.tsx'),
  ]),
  // The addresses settings had before they moved into the organization. They only forward (see the files).
  route('/settings', '../features/session/routes/legacy-settings.tsx'),
  route('/settings/appearance', '../features/session/routes/legacy-appearance.tsx'),
]);
