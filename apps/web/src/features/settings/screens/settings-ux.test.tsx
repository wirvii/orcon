// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, expect, it } from 'vitest';

import { flow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, currentUser, sessionId, signedIn } from '../../../../test/session-fixtures';

// The six sections of Settings (SPEC-019 UX9). A section whose server operation is not served says so plainly.

function signInAs(role?: string) {
  server.use(
    role === undefined
      ? signedIn.currentUser
      : http.get('/api/v1/users/me', () => HttpResponse.json({ ...currentUser, orgRoles: [{ orgId: acme.id, role }] })),
    signedIn.organizations([acme]),
    productApi.projects([project]),
    productApi.flows([flow]),
  );
}

afterEach(() => {
  document.documentElement.removeAttribute('data-motion');
  window.localStorage.clear();
});

it('settings menu groups your account and organization', async () => {
  signInAs();
  renderRoute('/orgs/acme/settings/profile');
  const nav = await screen.findByRole('navigation', { name: 'Settings sections' });
  expect(within(nav).getByText('Your account')).toBeInTheDocument();
  expect(within(nav).getByText('Organization · Acme')).toBeInTheDocument();
  expect(within(nav).getAllByRole('link').map((link) => link.textContent)).toEqual([
    'Profile',
    'Password and sessions',
    'Appearance',
    'General',
    'People and roles',
    'API keys',
  ]);
});

it('profile is read only until the update operation exists', async () => {
  signInAs();
  renderRoute('/orgs/acme/settings/profile');
  const card = (await screen.findByRole('heading', { level: 2, name: 'Profile' })).closest('section') as HTMLElement;
  expect(within(card).getByText('Ana Ruiz')).toBeVisible();
  expect(within(card).getByText('ana@example.com')).toBeVisible();
  expect(within(card).queryByRole('textbox')).toBeNull();
  expect(within(card).queryByRole('button', { name: /save/i })).toBeNull();
  expect(within(card).getByText(/ask whoever administers the organization/i)).toBeVisible();
});

it('security offers sign out of this session and coming soon for the rest', async () => {
  signInAs();
  let signedOut = false;
  server.use(
    http.post(`/api/v1/users/me/sessions/${sessionId}/revoke`, () => {
      signedOut = true;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  renderRoute('/orgs/acme/settings/security');
  const card = (await screen.findByRole('heading', { level: 2, name: 'This session' })).closest('section') as HTMLElement;
  expect(within(card).getByText('This browser')).toBeVisible();
  expect(within(card).getByText('Active now')).toBeVisible();

  const soon = screen.getByRole('heading', { level: 2, name: 'Change password and open sessions' }).closest('[data-coming-soon]') as HTMLElement;
  expect(within(soon).getByText('Coming soon')).toBeVisible();
  expect(within(soon).queryByRole('button')).toBeNull();
  expect(screen.queryByLabelText(/password/i)).toBeNull();
  expect(screen.queryByRole('alert')).toBeNull();

  await userEvent.click(within(card).getByRole('button', { name: 'Sign out of this session' }));
  await waitFor(() => {
    expect(signedOut).toBe(true);
  });
});

it('appearance applies theme density language and reduced motion instantly without a save button', async () => {
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

  const motion = screen.getByRole('switch', { name: 'Reduce animations' });
  expect(motion).toHaveAttribute('aria-checked', 'false');
  await userEvent.click(motion);
  expect(motion).toHaveAttribute('aria-checked', 'true');
  expect(document.documentElement).toHaveAttribute('data-motion', 'reduce');
  expect(window.localStorage.getItem('orcon.motion')).toBe('true');
  await userEvent.click(motion);
  expect(document.documentElement).not.toHaveAttribute('data-motion');

  await userEvent.click(screen.getByRole('radio', { name: 'Español' }));
  expect(await screen.findByRole('heading', { level: 2, name: 'Apariencia' })).toBeVisible();
  expect(document.documentElement).toHaveAttribute('lang', 'es');
});

it('appearance announces saved automatically politely', async () => {
  signInAs();
  renderRoute('/orgs/acme/settings/appearance');
  await screen.findByRole('heading', { level: 2, name: 'Appearance' });
  const status = screen.getByRole('status');
  expect(status).toHaveAttribute('aria-live', 'polite');
  expect(status).toBeEmptyDOMElement();
  await userEvent.click(screen.getByRole('switch', { name: 'Reduce animations' }));
  expect(status).toHaveTextContent('Saved automatically');
});

it('organization general is read only and hides the danger zone', async () => {
  signInAs();
  renderRoute('/orgs/acme/settings/organization');
  const card = (await screen.findByRole('heading', { level: 2, name: 'Organization' })).closest('section') as HTMLElement;
  expect(within(card).getByText('Acme')).toBeVisible();
  expect(within(card).getByText('acme')).toBeVisible();
  expect(within(card).queryByRole('textbox')).toBeNull();
  expect(screen.queryByText(/danger zone/i)).toBeNull();
  expect(screen.queryByRole('button', { name: /delete/i })).toBeNull();
});

it('members shows the approved organization roles and coming soon', async () => {
  signInAs('Admin');
  renderRoute('/orgs/acme/settings/members');
  const card = (await screen.findByRole('heading', { level: 2, name: 'People and roles' })).closest('section') as HTMLElement;
  const organizationRoles = within(within(card).getByRole('region', { name: 'Organization roles' })).getAllByRole('listitem');
  expect(organizationRoles.map((item) => within(item).getByText(/^(Owner|Administrator|Member)$/).textContent)).toEqual(['Owner', 'Administrator', 'Member']);
  const projectRoles = within(within(card).getByRole('region', { name: 'Project roles' })).getAllByRole('listitem');
  expect(projectRoles).toHaveLength(4);
  expect(within(card).getByText('Your role in Acme:').parentElement).toHaveTextContent('Administrator');

  const soon = screen.getByRole('heading', { level: 2, name: 'The people of the organization' }).closest('[data-coming-soon]') as HTMLElement;
  expect(within(soon).getByText('Coming soon')).toBeVisible();
  expect(within(soon).queryByRole('button')).toBeNull();
  expect(screen.queryByRole('alert')).toBeNull();
});

it('api keys shows coming soon until the operation exists', async () => {
  signInAs();
  renderRoute('/orgs/acme/settings/api-keys');
  const card = (await screen.findByRole('heading', { level: 2, name: 'API keys' })).closest('[data-coming-soon]') as HTMLElement;
  expect(within(card).getByText('Coming soon')).toBeVisible();
  expect(within(card).queryByRole('button')).toBeNull();
  expect(screen.queryByRole('alert')).toBeNull();
});
