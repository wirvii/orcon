// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
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
});
