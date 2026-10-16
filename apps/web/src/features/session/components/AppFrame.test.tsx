// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, expect, it, vi } from 'vitest';

import { flow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, currentUser, problem, sessionId, signedIn } from '../../../../test/session-fixtures';

// The frame of the signed-in pages (SPEC-019 UX1). The titles are the ones the acceptance criteria name.

// The server has no operation that counts the executions to review yet (SPEC-019 UXB3), so the hook that reads it
// is replaced to say what the sidebar draws once it exists.
const review = vi.hoisted(() => ({ count: undefined as number | undefined }));
vi.mock('../../../app/shell/needs-review', () => ({ useNeedsReviewCount: () => review.count }));

afterEach(() => {
  review.count = undefined;
});

const sales = { ...project, flowCount: 7 };
const support = { ...project, id: '0195f1a0-0000-7000-8000-0000000000d2', name: 'Soporte', slug: 'soporte', handle: 'soporte', flowCount: 4 };

/** The sidebar: the first navigation landmark. The tab bar of a phone is the last one. */
const sidebar = () => within(screen.getAllByRole('navigation', { name: 'Main navigation' })[0] as HTMLElement);

/** Renders the app at a path and waits for the frame: the sidebar is drawn once the session and the route are read. */
async function open(path: string) {
  const view = renderRoute(path);
  await screen.findAllByRole('navigation', { name: 'Main navigation' });
  return view;
}

function signInAs(projects: readonly { handle: string; [key: string]: unknown }[] = [sales, support]) {
  server.use(
    signedIn.currentUser,
    signedIn.organizations([acme]),
    productApi.projects(projects),
    productApi.flows([flow]),
    // The flows of any other project: none.
    http.get('/api/v1/projects/:projectId/flows', () => HttpResponse.json({ items: [] })),
  );
}

it('AppFrame shows the active organization name', async () => {
  signInAs();
  await open('/orgs/acme/overview');
  // The name of the organization is on the button at the top of the sidebar, with what it is under it.
  const button = await screen.findByRole('button', { name: 'Acme Organization' });
  expect(button).toBeVisible();
});

it('AppFrame lists only readable projects with their flow count', async () => {
  signInAs();
  await open('/orgs/acme/overview');
  const view = sidebar();
  // Each project the server returned (it returns only the ones the person can read) with its number of flows.
  expect(await view.findByRole('link', { name: 'Ventas 7' })).toHaveAttribute('href', '/orgs/acme/projects/ventas/flows');
  expect(view.getByRole('link', { name: 'Soporte 4' })).toHaveAttribute('href', '/orgs/acme/projects/soporte/flows');
  expect(view.queryByRole('link', { name: /Operaciones/ })).toBeNull();
});

it('AppFrame marks only the section of the current route', async () => {
  signInAs();
  const { router } = await open('/orgs/acme/overview');
  const view = sidebar();
  expect(await view.findByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
  expect(view.getByRole('link', { name: 'Ventas 7' })).not.toHaveAttribute('aria-current');
  expect(view.getByRole('link', { name: 'Templates' })).not.toHaveAttribute('aria-current');

  await userEvent.click(view.getByRole('link', { name: 'Ventas 7' }));
  await waitFor(() => {
    expect(router.state.location.pathname).toBe('/orgs/acme/projects/ventas/flows');
  });
  await waitFor(() => {
    expect(sidebar().getByRole('link', { name: 'Ventas 7' })).toHaveAttribute('aria-current', 'page');
  });
  expect(sidebar().getByRole('link', { name: 'Home' })).not.toHaveAttribute('aria-current');
  expect(sidebar().getByRole('link', { name: 'Soporte 4' })).not.toHaveAttribute('aria-current');
});

it('AppFrame hides create organization and whats new', async () => {
  signInAs();
  await open('/orgs/acme/overview');
  await userEvent.click(await screen.findByRole('button', { name: 'Acme Organization' }));
  const organizations = await screen.findByRole('menu');
  expect(within(organizations).getByRole('menuitemcheckbox', { name: 'Acme' })).toBeChecked();
  expect(within(organizations).getByRole('menuitem', { name: 'Organization settings' })).toHaveAttribute('href', '/orgs/acme/settings/organization');
  expect(within(organizations).queryByText(/create/i)).toBeNull();
  await userEvent.keyboard('{Escape}');

  await userEvent.click(sidebar().getByRole('button', { name: 'Help' }));
  const help = await screen.findByRole('menu');
  expect(within(help).getAllByRole('menuitem').map((item) => item.textContent)).toEqual([
    'Documentation(opens in a new tab)',
    'API reference',
    'Community and support(opens in a new tab)',
    'Guide for coming from n8n(opens in a new tab)',
    'Keyboard shortcuts?',
  ]);
  expect(within(help).queryByText(/what.s new/i)).toBeNull();
  // The API reference is a page of the server and opens in this tab; the others leave the app and open a new one.
  expect(within(help).getByRole('menuitem', { name: 'API reference' })).toHaveAttribute('href', '/api/reference/');
  expect(within(help).getByRole('menuitem', { name: 'API reference' })).not.toHaveAttribute('target');
  for (const name of [/^Documentation/, /^Community/, /^Guide/]) expect(within(help).getByRole('menuitem', { name })).toHaveAttribute('target', '_blank');
});

it('AppFrame shows the needs review count next to executions when greater than zero', async () => {
  review.count = 2;
  signInAs();
  await open('/orgs/acme/overview');
  const executions = await sidebar().findByRole('link', { name: 'Executions 2' });
  expect(executions).toHaveAttribute('href', '/orgs/acme/executions');
});

it('AppFrame shows nothing next to executions when the count is unknown', async () => {
  signInAs();
  await open('/orgs/acme/overview');
  const executions = await sidebar().findByRole('link', { name: 'Executions' });
  expect(executions).toHaveTextContent(/^Executions$/);
});

it('AppFrame keeps the sidebar and user menu on settings', async () => {
  signInAs();
  await open('/orgs/acme/settings/appearance');
  expect(await screen.findByRole('heading', { level: 2, name: 'Appearance' })).toBeVisible();
  expect(await sidebar().findByRole('link', { name: 'Settings' })).toHaveAttribute('aria-current', 'page');
  // The account is the real name of the person, with their initials, and never the placeholder.
  const account = sidebar().getByRole('button', { name: /Ana Ruiz/ });
  expect(account).toHaveTextContent('AR');
  expect(account).not.toHaveTextContent(/^Owner/);
  expect(sidebar().getByRole('button', { name: 'Acme Organization' })).toBeVisible();
});

it('AppFrame signs out to login with a notice', async () => {
  let revoked = false;
  signInAs();
  server.use(
    http.post(`/api/v1/users/me/sessions/${sessionId}/revoke`, () => {
      revoked = true;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  const { router } = await open('/orgs/acme/overview');
  await userEvent.click(await sidebar().findByRole('button', { name: /Ana Ruiz/ }));
  await userEvent.click(await screen.findByRole('menuitem', { name: 'Sign out' }));
  expect(await screen.findByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
  expect(revoked).toBe(true);
  expect(router.state.location.pathname).toBe('/login');
  expect(screen.getByText('You signed out.')).toBeVisible();
});

it('AppFrame shows the role of the person when the server says it', async () => {
  server.use(
    http.get('/api/v1/users/me', () => HttpResponse.json({ ...currentUser, orgRoles: [{ orgId: acme.id, role: 'Owner' }] })),
    signedIn.organizations([acme]),
    productApi.projects([sales]),
    productApi.flows([flow]),
  );
  await open('/orgs/acme/overview');
  expect(await sidebar().findByRole('button', { name: 'Ana Ruiz Owner · Acme' })).toBeVisible();
});

it('AppFrame offers the bottom bar of a phone with five labelled destinations', async () => {
  signInAs();
  await open('/orgs/acme/overview');
  await screen.findByRole('button', { name: 'Acme Organization' });
  const bar = within(screen.getAllByRole('navigation', { name: 'Main navigation' }).at(-1) as HTMLElement);
  expect(bar.getAllByRole('link').map((link) => link.textContent)).toEqual(['Home', 'Flows', 'Executions', 'Credentials', 'Settings']);
  expect(bar.getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
});

it('AppFrame sends a person without a session to sign-in', async () => {
  server.use(http.get('/api/v1/users/me', () => problem(401, 'unauthenticated', 'unauthenticated')));
  const { router } = renderRoute('/orgs/acme/overview');
  expect(await screen.findByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
  expect(router.state.location.pathname).toBe('/login');
});
