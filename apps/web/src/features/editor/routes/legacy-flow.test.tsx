// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { waitFor } from '@testing-library/react';
import { HttpResponse, http } from 'msw';
import { describe, expect, it } from 'vitest';

import { flow, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, signedIn } from '../../../../test/session-fixtures';

describe('the editor address before projects', () => {
  it('forwards to the flow under its organization and project', async () => {
    server.use(
      signedIn.currentUser,
      signedIn.organizations([acme]),
      http.get(`/api/v1/flows/${flow.id}`, () => HttpResponse.json(flow)),
      http.get(`/api/v1/projects/${project.id}`, () => HttpResponse.json(project)),
    );
    const { router } = renderRoute(`/flows/${flow.id}`);
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/orgs/acme/projects/${project.handle}/flows/${flow.id}`);
    });
  });

  it('stays where it is when the flow is not there for the person', async () => {
    server.use(
      signedIn.currentUser,
      signedIn.organizations([acme]),
      http.get(`/api/v1/flows/${flow.id}`, () => HttpResponse.json({ type: 'about:blank', title: 'not found', status: 404, code: 'not-found' }, { status: 404 })),
    );
    const { router } = renderRoute(`/flows/${flow.id}`);
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/flows/${flow.id}`);
    });
    expect(router.state.location.pathname).not.toContain('/orgs/');
  });
});
