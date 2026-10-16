// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http } from 'msw';
import { describe, expect, it } from 'vitest';

import { flow, productApi, project } from '../../../test/product-fixtures';
import { renderRoute, server } from '../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../test/session-fixtures';

function signInAs() {
  server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]), productApi.flows([flow]));
}

describe('the organization pages whose whole-organization view does not exist yet', () => {
  it('says templates are coming and offers no action that does nothing', async () => {
    signInAs();
    renderRoute('/orgs/acme/templates');
    const card = (await screen.findByRole('heading', { level: 2, name: 'Templates' })).closest('[data-coming-soon]') as HTMLElement;
    expect(within(card).getByText('Coming soon')).toBeVisible();
    expect(within(card).queryByRole('button')).toBeNull();
    // Not an error: nothing failed.
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('leads credentials to the credentials of each project, which exist', async () => {
    signInAs();
    const { router } = renderRoute('/orgs/acme/credentials');
    expect(await screen.findByRole('heading', { level: 1, name: 'Credentials' })).toBeVisible();
    const list = (await screen.findByRole('heading', { name: 'Meanwhile, open a project' })).closest('section') as HTMLElement;
    const row = await within(list).findByRole('link', { name: 'Ventas' });
    expect(row).toHaveAttribute('href', '/orgs/acme/projects/ventas/credentials');
    await userEvent.click(row);
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/orgs/acme/projects/ventas/credentials');
    });
  });

  it('leads executions to the executions of each project', async () => {
    signInAs();
    renderRoute('/orgs/acme/executions');
    const list = (await screen.findByRole('heading', { name: 'Meanwhile, open a project' })).closest('section') as HTMLElement;
    expect(await within(list).findByRole('link', { name: 'Ventas' })).toHaveAttribute('href', '/orgs/acme/projects/ventas/executions');
  });

  it('says there are no projects when there is none to open', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([]));
    renderRoute('/orgs/acme/credentials');
    expect(await screen.findByRole('heading', { name: 'No projects yet' })).toBeVisible();
  });

  it('shows a problem when the projects cannot be read', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), http.get(`/api/v1/orgs/${acme.id}/projects`, () => problem(500, 'Internal Server Error', 'internal')));
    renderRoute('/orgs/acme/executions');
    expect(await screen.findByRole('alert')).toBeVisible();
  });
});
