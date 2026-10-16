// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, expect, it } from 'vitest';

import { flow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, currentUser, problem, sessionId, signedIn, userEtag } from '../../../../test/session-fixtures';

// The six sections of Settings (SPEC-019 UX9). A section whose server operation is not served says so plainly.

function signInAs(role?: string) {
  server.use(
    role === undefined
      ? signedIn.currentUser
      : http.get('/api/v1/users/me', () => HttpResponse.json({ ...currentUser, orgRoles: [{ orgId: acme.id, role }] }, { headers: { ETag: userEtag } })),
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

const otherSession = {
  id: '0195f1a0-0000-7000-8000-0000000000a2',
  createdAt: '2026-10-12T09:30:00Z',
  expiresAt: '2026-11-11T09:30:00Z',
  idleExpiresAt: '2026-10-12T17:30:00Z',
  assuranceLevel: 'AAL1',
};

it('profile lets the person change the name with the entity tag and refreshes the frame', async () => {
  signInAs();
  let ifMatch: string | null = null;
  let body: unknown;
  let renamed = false;
  server.use(
    http.get('/api/v1/users/me', () =>
      HttpResponse.json({ ...currentUser, displayName: renamed ? 'Ana Torres' : currentUser.displayName }, { headers: { ETag: renamed ? '"2"' : userEtag } }),
    ),
    http.patch('/api/v1/users/me', async ({ request }) => {
      ifMatch = request.headers.get('If-Match');
      body = await request.json();
      renamed = true;
      return HttpResponse.json({ ...currentUser, displayName: 'Ana Torres' }, { headers: { ETag: '"2"' } });
    }),
  );
  renderRoute('/orgs/acme/settings/profile');
  const field = await screen.findByRole('textbox', { name: /Name/ });
  expect(field).toHaveValue('Ana Ruiz');
  expect(screen.getByText('ana@example.com')).toBeVisible();
  await userEvent.clear(field);
  await userEvent.type(field, 'Ana Torres');
  await userEvent.click(screen.getByRole('button', { name: 'Save name' }));
  expect(await screen.findByText('Name saved.')).toBeInTheDocument();
  expect(ifMatch).toBe(userEtag);
  expect(body).toEqual({ displayName: 'Ana Torres' });
  // The frame reads the answer: the sidebar shows the new name without a reload.
  expect((await screen.findAllByText('Ana Torres')).length).toBeGreaterThan(0);
});

it('profile explains a refused change and loads the current name', async () => {
  signInAs();
  let current = 'Ana Ruiz';
  server.use(
    http.get('/api/v1/users/me', () => HttpResponse.json({ ...currentUser, displayName: current }, { headers: { ETag: '"2"' } })),
    http.patch('/api/v1/users/me', () => problem(412, 'Precondition failed', 'precondition-failed')),
  );
  renderRoute('/orgs/acme/settings/profile');
  const field = await screen.findByRole('textbox', { name: /Name/ });
  current = 'Ana Maria';
  await userEvent.clear(field);
  await userEvent.type(field, 'Ana Torres');
  await userEvent.click(screen.getByRole('button', { name: 'Save name' }));
  expect(await screen.findByText(/Someone else changed this account/)).toBeVisible();
  await userEvent.click(screen.getByRole('button', { name: 'Load the current name' }));
  await waitFor(() => {
    expect(screen.getByRole('textbox', { name: /Name/ })).toHaveValue('Ana Maria');
  });
  expect(screen.queryByText(/Someone else changed this account/)).toBeNull();
});

it('profile treats the name Owner as unset and asks for a real one', async () => {
  server.use(
    http.get('/api/v1/users/me', () => HttpResponse.json({ ...currentUser, displayName: 'Owner' }, { headers: { ETag: userEtag } })),
    signedIn.organizations([acme]),
    productApi.projects([project]),
    productApi.flows([flow]),
  );
  renderRoute('/orgs/acme/settings/profile');
  expect(await screen.findByRole('textbox', { name: /Name/ })).toHaveValue('');
  await userEvent.click(screen.getByRole('button', { name: 'Save name' }));
  expect(await screen.findByText('This field is required.')).toBeVisible();
});

it('security signs out this session, lists and signs out the others and says the password is coming', async () => {
  signInAs();
  let signedOut = false;
  const revoked: string[] = [];
  let sessions = [currentUser.currentSession, otherSession];
  server.use(
    http.get('/api/v1/users/me/sessions', () => HttpResponse.json({ items: sessions })),
    http.post(`/api/v1/users/me/sessions/${sessionId}/revoke`, () => {
      signedOut = true;
      return new HttpResponse(null, { status: 204 });
    }),
    http.post(`/api/v1/users/me/sessions/${otherSession.id}/revoke`, () => {
      revoked.push(otherSession.id);
      sessions = [currentUser.currentSession];
      return new HttpResponse(null, { status: 204 });
    }),
  );
  renderRoute('/orgs/acme/settings/security');
  const card = (await screen.findByRole('heading', { level: 2, name: 'This session' })).closest('section') as HTMLElement;
  expect(within(card).getByText('This browser')).toBeVisible();
  expect(within(card).getByText('Active now')).toBeVisible();

  const others = (await screen.findByRole('heading', { level: 2, name: 'Other sessions' })).closest('section') as HTMLElement;
  expect(await within(others).findByText('Another browser or device')).toBeVisible();
  expect(within(others).getAllByRole('listitem')).toHaveLength(1);
  await userEvent.click(within(others).getByRole('button', { name: /Sign out the session started/ }));
  await waitFor(() => {
    expect(revoked).toEqual([otherSession.id]);
  });
  expect(await within(others).findByText('You have no other open sessions.')).toBeVisible();
  expect(signedOut).toBe(false);

  const soon = screen.getByRole('heading', { level: 2, name: 'Change password' }).closest('[data-coming-soon]') as HTMLElement;
  expect(within(soon).getByText('Coming soon')).toBeVisible();
  expect(within(soon).getByText(/orcon accounts set-password/)).toBeVisible();
  expect(within(soon).queryByRole('button')).toBeNull();
  expect(screen.queryByLabelText(/password/i)).toBeNull();

  await userEvent.click(within(card).getByRole('button', { name: 'Sign out of this session' }));
  await waitFor(() => {
    expect(signedOut).toBe(true);
  });
});

it('security shows an error with a retry when the sessions cannot be listed', async () => {
  signInAs();
  server.use(http.get('/api/v1/users/me/sessions', () => problem(500, 'Internal error', 'internal-error')));
  renderRoute('/orgs/acme/settings/security');
  const others = (await screen.findByRole('heading', { level: 2, name: 'Other sessions' })).closest('section') as HTMLElement;
  expect(await within(others).findByRole('button', { name: 'Try again' })).toBeVisible();
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
