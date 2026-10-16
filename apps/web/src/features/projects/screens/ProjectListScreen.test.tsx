// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, within } from '@testing-library/react';
import { http } from 'msw';
import { describe, expect, it } from 'vitest';

import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';
import { productApi, project } from '../../../../test/product-fixtures';

describe('ProjectListScreen', () => {
  it('loading', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), http.get(`/api/v1/orgs/${acme.id}/projects`, () => new Promise(() => undefined)));
    renderRoute('/orgs/acme/projects');
    expect(await screen.findByRole('status', { name: 'Loading' })).toBeVisible();
  });

  it('empty', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([]));
    renderRoute('/orgs/acme/projects');
    expect(await screen.findByRole('heading', { name: 'No projects yet' })).toBeVisible();
  });

  it('error', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), http.get(`/api/v1/orgs/${acme.id}/projects`, () => problem(500, 'internal', 'internal')));
    renderRoute('/orgs/acme/projects');
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
  });

  it('forbidden', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), http.get(`/api/v1/orgs/${acme.id}/projects`, () => problem(403, 'forbidden', 'forbidden')));
    renderRoute('/orgs/acme/projects');
    expect(await screen.findByText('You do not have permission to see this')).toBeVisible();
  });

  it('data', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]));
    renderRoute('/orgs/acme/projects');
    expect(await screen.findByRole('link', { name: 'Ventas' })).toHaveAttribute('href', '/orgs/acme/projects/ventas/flows');
    expect(screen.getByRole('cell', { name: 'ventas' })).toBeVisible();
    expect(screen.getByRole('cell', { name: 'production' })).toBeVisible();
  });

  it('shows how many flows each project has, and nothing when the server does not say', async () => {
    const empty = { ...project, id: '0195f1a0-0000-7000-8000-0000000000d2', name: 'Soporte', handle: 'soporte', slug: 'soporte' };
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([{ ...project, flowCount: 7 }, empty]));
    renderRoute('/orgs/acme/projects');
    const table = await screen.findByRole('table');
    const row = within(table).getByRole('link', { name: 'Ventas' }).closest('tr') as HTMLElement;
    expect(within(row).getByRole('cell', { name: '7' })).toBeVisible();
    // An unknown count is left blank, never drawn as zero.
    const other = within(table).getByRole('link', { name: 'Soporte' }).closest('tr') as HTMLElement;
    expect(within(other).queryByRole('cell', { name: '0' })).toBeNull();
  });
});
