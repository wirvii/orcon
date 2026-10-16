// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { productApi } from '../../../../test/product-fixtures';
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
    expect(screen.getByLabelText(/^Password/)).toHaveAttribute('type', 'password');
    // Forgetting the password has a link next to its label; it opens the recovery panel in the same page.
    expect(screen.getByRole('link', { name: 'Forgot it?' })).toHaveAttribute('href', '/login?step=forgot-password');
    // The brand side says why Orcon, in three reasons; it is explanation, not a control.
    expect(screen.getByRole('heading', { level: 2, name: /Build, orchestrate and execute your way\./ })).toBeInTheDocument();
    expect(screen.getByText('Bring your n8n flows: we import them and tell you what changed.')).toBeInTheDocument();
    expect(screen.getByText('A step that sends something out never sends it twice.')).toBeInTheDocument();
    expect(screen.getByText('Open source (Apache-2.0). Your data, on your server.')).toBeInTheDocument();
    // Accounts are made by an administrator: this is text, not a link.
    expect(screen.getByText(/Ask whoever administers your organization for an invitation\./)).toBeVisible();
  });

  it('has no SSO button', async () => {
    renderRoute('/login');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    expect(screen.queryByRole('button', { name: /SSO|single sign/i })).toBeNull();
    expect(screen.queryByRole('link', { name: /SSO|single sign/i })).toBeNull();
    expect(screen.queryByText(/SSO/)).toBeNull();
  });

  it('language and theme apply instantly', async () => {
    renderRoute('/login');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    // Theme: one button flips between dark and light and the page changes at once.
    await userEvent.click(screen.getByRole('button', { name: 'Switch to the dark theme' }));
    await waitFor(() => {
      expect(document.documentElement).toHaveAttribute('data-theme', 'dark');
    });
    expect(window.localStorage.getItem('orcon.theme')).toBe('dark');
    await userEvent.click(screen.getByRole('button', { name: 'Switch to the light theme' }));
    await waitFor(() => {
      expect(document.documentElement).toHaveAttribute('data-theme', 'light');
    });
    // Language: the button shows the language in use and the page is translated the moment another is chosen.
    await userEvent.click(screen.getByRole('button', { name: 'English' }));
    await userEvent.click(await screen.findByRole('menuitemcheckbox', { name: 'Español' }));
    expect(await screen.findByRole('heading', { level: 1, name: 'Iniciar sesión' })).toBeVisible();
    expect(document.documentElement).toHaveAttribute('lang', 'es');
    expect(window.localStorage.getItem('orcon.locale')).toBe('es');
    expect(screen.getByRole('heading', { level: 2, name: /Construye, orquesta y ejecuta a tu manera\./ })).toBeInTheDocument();
  });

  it('shows the password only while the eye is pressed', async () => {
    renderRoute('/login');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    await userEvent.type(screen.getByLabelText(/^Password/), 'secret-password-1');
    const eye = screen.getByRole('button', { name: 'Show password' });
    expect(eye).toHaveAttribute('aria-pressed', 'false');
    await userEvent.click(eye);
    expect(screen.getByLabelText(/^Password/)).toHaveAttribute('type', 'text');
    await userEvent.click(screen.getByRole('button', { name: 'Hide password' }));
    expect(screen.getByLabelText(/^Password/)).toHaveAttribute('type', 'password');
  });

  it('opens the recovery panel from the link and comes back with the email kept', async () => {
    const { router } = renderRoute('/login');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    await userEvent.type(screen.getByLabelText(/^Email/), 'ana@example.com');
    await userEvent.click(screen.getByRole('link', { name: 'Forgot it?' }));
    expect(await screen.findByRole('heading', { level: 1, name: 'Recover your password' })).toBeVisible();
    expect(router.state.location.search).toEqual({ step: 'forgot-password' });
    await userEvent.click(screen.getByRole('button', { name: 'Back' }));
    expect(await screen.findByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
    expect(screen.getByLabelText(/^Email/)).toHaveValue('ana@example.com');
  });

  it('says so, without moving the form, after signing out', async () => {
    renderRoute('/login?signedOut=true');
    expect(await screen.findByText('You signed out.')).toBeVisible();
    expect(screen.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
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
    // The notice says what to do next and sits above the fields.
    expect(screen.getByText('Check your details or recover your password.')).toBeVisible();
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
    server.use(signedIn.createSession(), signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([]));
    const { router } = renderRoute('/login?redirect=%2Forgs%2Facme%2Fsettings%2Fappearance');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    await fillForm();
    expect(await screen.findByRole('heading', { level: 2, name: 'Appearance' })).toBeVisible();
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/orgs/acme/settings/appearance');
    });
  });
});
