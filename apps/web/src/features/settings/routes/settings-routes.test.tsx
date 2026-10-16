// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http } from 'msw';
import { expect, it, vi } from 'vitest';

import { flow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, globex, problem, signedIn } from '../../../../test/session-fixtures';
import { FIRST_USEFUL_MARK } from '../../../app/perf/marks';

// Settings inside the organization (SPEC-019 Q4). The titles are the ones the acceptance criteria name.

function signInAs() {
  server.use(
    signedIn.currentUser,
    signedIn.organizations([acme]),
    productApi.projects([project]),
    productApi.flows([flow]),
  );
}

/** Each section: its address, the name of its link in the list and the heading of its first card. */
const sections = [
  ['profile', 'Profile', 'Profile'],
  ['security', 'Password and sessions', 'This session'],
  ['appearance', 'Appearance', 'Appearance'],
  ['organization', 'General', 'Organization'],
  ['members', 'People and roles', 'People and roles'],
  ['api-keys', 'API keys', 'API keys'],
] as const;

it('settings routes live under the organization', async () => {
  signInAs();
  const { router } = renderRoute('/orgs/acme/settings');
  // The address of Settings alone opens the first section.
  expect(await screen.findByRole('heading', { level: 1, name: 'Settings' })).toBeVisible();
  await waitFor(() => {
    expect(router.state.location.pathname).toBe('/orgs/acme/settings/profile');
  });

  // Every section is an address under the organization and shows its own content in the same page.
  const nav = screen.getByRole('navigation', { name: 'Settings sections' });
  for (const [path, link, heading] of sections) {
    await userEvent.click(within(nav).getByRole('link', { name: link }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/orgs/acme/settings/${path}`);
    });
    expect(await screen.findByRole('heading', { level: 2, name: heading })).toBeVisible();
    // Only the section being shown is marked.
    expect(within(nav).getAllByRole('link').filter((link) => link.getAttribute('aria-current') === 'page')).toHaveLength(1);
  }
});

it('legacy appearance route redirects to the organization settings', async () => {
  signInAs();
  const { router } = renderRoute('/settings/appearance');
  expect(await screen.findByRole('heading', { level: 2, name: 'Appearance' })).toBeVisible();
  expect(router.state.location.pathname).toBe('/orgs/acme/settings/appearance');
  // The frame is the one of every signed-in page.
  expect(screen.getAllByRole('navigation', { name: 'Main navigation' }).length).toBeGreaterThan(0);
});

it('legacy appearance route sends a person without a session to sign-in', async () => {
  server.use(http.get('/api/v1/users/me', () => problem(401, 'unauthenticated', 'unauthenticated')));
  const { router } = renderRoute('/settings/appearance');
  expect(await screen.findByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
  expect(router.state.location.pathname).toBe('/login');
});

it('legacy appearance route sends a person with several organizations to the picker when none is remembered', async () => {
  server.use(signedIn.currentUser, signedIn.organizations([acme, globex]));
  const { router } = renderRoute('/settings/appearance');
  expect(await screen.findByRole('heading', { level: 1, name: 'Choose an organization' })).toBeVisible();
  expect(router.state.location.pathname).toBe('/orgs');
});

it('legacy settings address opens the profile', async () => {
  signInAs();
  const { router } = renderRoute('/settings');
  expect(await screen.findByRole('heading', { level: 2, name: 'Profile' })).toBeVisible();
  expect(router.state.location.pathname).toBe('/orgs/acme/settings/profile');
});

it('the appearance choices apply at once and are announced politely, with no save button', async () => {
  signInAs();
  renderRoute('/orgs/acme/settings/appearance');
  await screen.findByRole('heading', { level: 2, name: 'Appearance' });
  expect(screen.queryByRole('button', { name: /save/i })).toBeNull();

  await userEvent.click(screen.getByRole('radio', { name: 'Light' }));
  await waitFor(() => {
    expect(document.documentElement).toHaveAttribute('data-theme', 'light');
  });
  await userEvent.click(screen.getByRole('radio', { name: 'Compact' }));
  expect(document.documentElement).toHaveAttribute('data-density', 'compact');
  expect(screen.getByRole('status')).toHaveTextContent('Saved automatically');

  await userEvent.click(screen.getByRole('radio', { name: 'Español' }));
  expect(await screen.findByRole('heading', { level: 2, name: 'Apariencia' })).toBeVisible();
  expect(document.documentElement).toHaveAttribute('lang', 'es');
});

it('the profile shows the real name and email, read only', async () => {
  signInAs();
  renderRoute('/orgs/acme/settings/profile');
  const card = (await screen.findByRole('heading', { level: 2, name: 'Profile' })).closest('section') as HTMLElement;
  expect(within(card).getByText('Ana Ruiz')).toBeVisible();
  expect(within(card).getByText('ana@example.com')).toBeVisible();
  expect(within(card).queryByRole('textbox')).toBeNull();
});

it('a section that needs a server part that does not exist yet says so instead of failing', async () => {
  signInAs();
  renderRoute('/orgs/acme/settings/members');
  const card = (await screen.findByRole('heading', { level: 2, name: 'People and roles' })).closest('[data-coming-soon]') as HTMLElement;
  expect(within(card).getByText('Coming soon')).toBeVisible();
  expect(within(card).queryByRole('button')).toBeNull();
  expect(screen.queryByRole('alert')).toBeNull();
});

it('marks the first useful screen of a settings page', async () => {
  signInAs();
  const mark = vi.spyOn(performance, 'mark');
  renderRoute('/orgs/acme/settings/appearance');
  await screen.findByRole('heading', { level: 2, name: 'Appearance' });
  expect(mark).toHaveBeenCalledWith(FIRST_USEFUL_MARK, { detail: { route: '/orgs/$orgSlug/settings/appearance' } });
  mark.mockRestore();
});
