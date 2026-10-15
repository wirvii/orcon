// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import { http } from 'msw';
import { describe, expect, it } from 'vitest';

import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, globex, problem, signedIn } from '../../../../test/session-fixtures';

describe('OrganizationPickerScreen', () => {
  it('loading', async () => {
    server.use(signedIn.currentUser, http.get('/api/v1/orgs', () => new Promise(() => undefined)));
    renderRoute('/orgs');
    expect(await screen.findByRole('status', { name: 'Loading' })).toBeVisible();
  });

  it('empty', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([]));
    renderRoute('/orgs');
    expect(await screen.findByRole('heading', { name: 'No organizations yet' })).toBeVisible();
  });

  it('error', async () => {
    server.use(signedIn.currentUser, http.get('/api/v1/orgs', () => problem(500, 'internal', 'internal')));
    renderRoute('/orgs');
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
  });

  it('forbidden', async () => {
    server.use(signedIn.currentUser, http.get('/api/v1/orgs', () => problem(403, 'forbidden', 'forbidden')));
    renderRoute('/orgs');
    expect(await screen.findByText('You do not have permission to see this')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
  });

  it('data', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme, globex]));
    renderRoute('/orgs');
    expect(await screen.findByRole('heading', { level: 1, name: 'Choose an organization' })).toBeVisible();
    expect(await screen.findByRole('link', { name: /Acme/ })).toHaveAttribute('href', '/orgs/acme');
    expect(screen.getByRole('link', { name: /Globex/ })).toHaveAttribute('href', '/orgs/globex');
  });
});
