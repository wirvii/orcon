// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { productApi } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, currentUser, globex, problem, sessionId, signedIn } from '../../../../test/session-fixtures';

describe('the session area routes', () => {
  it('sends a person without a session to sign-in, with the address they asked for', async () => {
    server.use(http.get('/api/v1/users/me', () => problem(401, 'unauthenticated', 'unauthenticated')));
    const { router } = renderRoute('/orgs');
    expect(await screen.findByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/login');
    expect(decodeURIComponent(router.state.location.href)).toContain('redirect=/orgs');
  });

  it('sends a person to sign-in when a later request answers 401, not only at the first load', async () => {
    server.use(signedIn.currentUser, http.get('/api/v1/orgs', () => problem(401, 'unauthenticated', 'unauthenticated')));
    const { router } = renderRoute('/orgs');
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/login');
    });
    expect(decodeURIComponent(router.state.location.href)).toContain('redirect=/orgs');
  });

  it('lands a person with no organization on the explanation', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([]));
    const { router } = renderRoute('/');
    expect(await screen.findByRole('heading', { name: 'Your account is not in an organization yet' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/');
  });

  it('lands a person with one organization on its home page', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([]));
    const { router } = renderRoute('/');
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/orgs/acme/overview');
    });
  });

  it('lands a person with several organizations on the picker', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme, globex]));
    const { router } = renderRoute('/');
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/orgs');
    });
  });

  it('says the page does not exist for an organization the person is not in', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]));
    renderRoute('/orgs/initech');
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeVisible();
  });

  it('signs out, goes to sign-in and forgets what it had read', async () => {
    let revoked = false;
    server.use(
      signedIn.currentUser,
      signedIn.organizations([acme, globex]),
      http.post(`/api/v1/users/me/sessions/${sessionId}/revoke`, () => {
        revoked = true;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { queryClient } = renderRoute('/orgs');
    await screen.findByRole('heading', { level: 1, name: 'Choose an organization' });
    await userEvent.click(screen.getByRole('button', { name: 'Ana Ruiz Account menu' }));
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Sign out' }));

    expect(await screen.findByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
    expect(revoked).toBe(true);
    expect(screen.getByText('You signed out.')).toBeVisible();
    expect(queryClient.getQueryCache().findAll({ queryKey: ['session'] })).toHaveLength(0);
  });

  it('shows the name derived from the email when the account has only the placeholder name', async () => {
    server.use(
      http.get('/api/v1/users/me', () => HttpResponse.json({ ...currentUser, email: 'jftamayop@example.com', displayName: 'Owner' })),
      signedIn.organizations([acme]),
      productApi.projects([]),
    );
    renderRoute('/orgs/acme');
    // The sidebar button and the avatar of the phone bar are both named after the person.
    const [button] = await screen.findAllByRole('button', { name: /^jftamayop/ });
    if (!button) throw new Error('The frame has no account button.');
    expect(button).toHaveTextContent('JF');
    await userEvent.click(button);
    const menu = await screen.findByRole('menu');
    expect(within(menu).getByText('jftamayop')).toBeVisible();
    expect(screen.queryByText('Owner')).toBeNull();
  });

  it('offers switching organization from the organization button of the sidebar', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme, globex]), productApi.projects([]));
    const { router } = renderRoute('/orgs/acme/overview');
    await userEvent.click(await screen.findByRole('button', { name: 'Acme Organization' }));
    const menu = await screen.findByRole('menu');
    expect(within(menu).getAllByRole('menuitemcheckbox')).toHaveLength(2);
    expect(within(menu).getByRole('menuitemcheckbox', { name: 'Acme' })).toBeChecked();
    expect(within(menu).getByRole('menuitemcheckbox', { name: 'Globex' })).not.toBeChecked();
    await userEvent.click(within(menu).getByRole('menuitemcheckbox', { name: 'Globex' }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/orgs/globex/overview');
    });
  });

  it('offers switching organization in the account menu only outside an organization and to a person with more than one', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]));
    const { unmount } = renderRoute('/orgs');
    await userEvent.click(await screen.findByRole('button', { name: 'Ana Ruiz Account menu' }));
    expect(await screen.findByRole('menuitem', { name: 'Sign out' })).toBeVisible();
    expect(screen.queryByRole('menuitem', { name: 'Switch organization' })).toBeNull();
    unmount();

    server.use(signedIn.organizations([acme, globex]));
    renderRoute('/orgs');
    await userEvent.click(await screen.findByRole('button', { name: 'Ana Ruiz Account menu' }));
    expect(await screen.findByRole('menuitem', { name: 'Switch organization' })).toBeVisible();
  });
});
