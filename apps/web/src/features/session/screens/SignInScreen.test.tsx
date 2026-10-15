// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';

async function fillForm(password = 'a-long-enough-password') {
  await userEvent.type(screen.getByLabelText(/^Email/), 'ana@example.com');
  await userEvent.type(screen.getByLabelText(/^Password/), password);
  await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));
}

describe('SignInScreen', () => {
  it('default', async () => {
    renderRoute('/login');
    expect(await screen.findByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Sign in' })).not.toHaveAttribute('aria-busy');
    expect(screen.getByRole('link', { name: 'Language and appearance' })).toHaveAttribute('href', '/settings/appearance');
    expect(screen.getByLabelText(/^Password/)).toHaveAttribute('type', 'password');
  });

  it('submitting', async () => {
    server.use(
      signedIn.createSession(async () => {
        await new Promise(() => undefined);
        return HttpResponse.json({}, { status: 201 });
      }),
    );
    renderRoute('/login');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    await fillForm();
    expect(await screen.findByRole('button', { name: 'Sign in' })).toHaveAttribute('aria-busy', 'true');
  });

  it('invalid credentials', async () => {
    server.use(signedIn.createSession(() => problem(401, 'unauthenticated', 'unauthenticated')));
    renderRoute('/login');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    await fillForm();
    expect(await screen.findByText('Email or password is incorrect.')).toBeVisible();
    // The password is cleared after a failure; the email stays.
    expect(screen.getByLabelText(/^Password/)).toHaveValue('');
    expect(screen.getByLabelText(/^Email/)).toHaveValue('ana@example.com');
  });

  it('throttled', async () => {
    server.use(signedIn.createSession(() => problem(429, 'rate-limited', 'rate-limited', { 'Retry-After': '300' })));
    renderRoute('/login');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    await fillForm();
    expect(await screen.findByText('Too many attempts. Wait 5 minutes and try again.')).toBeVisible();
  });

  it('server error', async () => {
    server.use(signedIn.createSession(() => problem(500, 'internal', 'internal')));
    renderRoute('/login');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    await fillForm();
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.queryByText('Email or password is incorrect.')).toBeNull();
  });

  it('returns to the page it came from after signing in', async () => {
    server.use(signedIn.createSession(), signedIn.currentUser, signedIn.organizations([acme]));
    const { router } = renderRoute('/login?redirect=%2Fsettings%2Fappearance');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    await fillForm();
    expect(await screen.findByRole('heading', { level: 1, name: 'Appearance and language' })).toBeVisible();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/settings/appearance');
    });
  });
});
