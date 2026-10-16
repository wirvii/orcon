// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http } from 'msw';
import { describe, expect, it } from 'vitest';

import { listedFlow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';

const base = '/orgs/acme/projects/ventas';

const beforeProject = () => [signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([{ ...project, flowCount: 7 }])];

describe('ProjectScreen', () => {
  it('loading', async () => {
    server.use(...beforeProject(), http.get(`/api/v1/orgs/${acme.id}/flows`, () => new Promise(() => undefined)));
    renderRoute(`${base}/flows`);
    expect(await screen.findByRole('status', { name: 'Loading' })).toBeVisible();
    // The name and the tabs are there while the flows load.
    expect(screen.getByRole('heading', { level: 1, name: 'Ventas' })).toBeVisible();
  });

  it('empty shows the thinking mascot and three exits', async () => {
    server.use(...beforeProject(), productApi.orgFlows([]));
    renderRoute(`${base}/flows`);
    const title = await screen.findByRole('heading', { name: 'Ventas is ready for its first flow' });
    expect(document.querySelector('[data-mascot-pose="thinking"]')).not.toBeNull();
    const exits = title.parentElement as HTMLElement;
    expect(within(exits).getByRole('button', { name: 'Create flow' })).toBeVisible();
    expect(within(exits).getByRole('button', { name: 'Import from n8n' })).toBeVisible();
    expect(within(exits).getByRole('button', { name: 'See templates' })).toBeVisible();
    // Each exit opens the chooser on its option.
    await userEvent.click(within(exits).getByRole('button', { name: 'Import from n8n' }));
    const dialog = await screen.findByRole('dialog', { name: 'Create flow' });
    expect(within(dialog).getByRole('button', { name: /Import from n8n/ })).toHaveAttribute('aria-pressed', 'true');
  });

  it('error', async () => {
    server.use(...beforeProject(), http.get(`/api/v1/orgs/${acme.id}/flows`, () => problem(500, 'internal', 'internal')));
    renderRoute(`${base}/flows`);
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
  });

  it('forbidden', async () => {
    server.use(...beforeProject(), http.get(`/api/v1/orgs/${acme.id}/flows`, () => problem(403, 'forbidden', 'forbidden')));
    renderRoute(`${base}/flows`);
    expect(await screen.findByText('You do not have permission to see this')).toBeVisible();
  });

  it('data', async () => {
    server.use(...beforeProject(), productApi.orgFlows([listedFlow]));
    const { router } = renderRoute(`${base}/flows`);
    expect(await screen.findByRole('link', { name: 'Order notifications' })).toBeVisible();
    expect(screen.getByRole('heading', { level: 1, name: 'Ventas' })).toBeVisible();

    // The four tabs, with the flow count the server sends; the people of the project are not shown (no contract).
    const tabs = screen.getByRole('navigation', { name: 'Sections of the project' });
    expect(within(tabs).getAllByRole('link').map((link) => link.textContent)).toEqual(['Flows7', 'Credentials', 'Executions', 'Settings']);
    expect(within(tabs).getByRole('link', { name: /Flows/ })).toHaveAttribute('aria-current', 'page');
    expect(screen.queryByTitle(/people/i)).toBeNull();

    // A tab leads to the page of that section, inside the same header.
    await userEvent.click(within(tabs).getByRole('link', { name: 'Settings' }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`${base}/settings`);
    });
    expect(await screen.findByText('production')).toBeVisible();
    expect(screen.getByRole('heading', { level: 1, name: 'Ventas' })).toBeVisible();
  });

  it('credentials tab shows coming soon when the operation answers 503', async () => {
    server.use(...beforeProject(), http.get(`/api/v1/projects/${project.id}/credentials`, () => problem(503, 'Service Unavailable', 'service-unavailable')));
    renderRoute(`${base}/credentials`);
    expect(await screen.findByRole('heading', { name: 'Credentials will be available soon' })).toBeVisible();
    expect(document.querySelector('[data-coming-soon]')).not.toBeNull();
    // Nothing failed: there is no error banner.
    expect(screen.queryByRole('alert')).toBeNull();
    // The header and the tabs stay.
    expect(within(screen.getByRole('navigation', { name: 'Sections of the project' })).getByRole('link', { name: 'Credentials' })).toHaveAttribute('aria-current', 'page');
  });

  it('does not draw the header on a page inside the project', async () => {
    server.use(...beforeProject(), productApi.orgFlows([listedFlow]));
    renderRoute(`${base}/flows`);
    await screen.findByRole('link', { name: 'Order notifications' });
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
  });
});
