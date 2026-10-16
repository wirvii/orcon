// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { expect, it } from 'vitest';

import { flow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, currentUser, signedIn } from '../../../../test/session-fixtures';

// The keyboard of the frame: collapsing the sidebar, creating a flow and the list of shortcuts (SPEC-019 R-UX-GEN-06).

const support = { ...project, id: '0195f1a0-0000-7000-8000-0000000000d2', name: 'Soporte', slug: 'soporte', handle: 'soporte' };

async function open(path: string) {
  server.use(
    signedIn.currentUser,
    signedIn.organizations([acme]),
    productApi.projects([project, support]),
    productApi.flows([flow]),
    http.get('/api/v1/projects/:projectId/flows', () => HttpResponse.json({ items: [] })),
  );
  const view = renderRoute(path);
  // The frame is drawn and the projects are read: only then does the key N know where to start a flow.
  await within((await screen.findAllByRole('navigation', { name: 'Main navigation' }))[0] as HTMLElement).findByRole('link', { name: 'Soporte' });
  return view;
}

it('the sidebar collapses with the bracket key and remembers the choice for that person', async () => {
  const first = await open('/orgs/acme/overview');
  const nav = screen.getAllByRole('navigation', { name: 'Main navigation' })[0] as HTMLElement;
  expect(nav).not.toHaveAttribute('data-collapsed');
  await userEvent.keyboard('[[');
  expect(nav).toHaveAttribute('data-collapsed', 'true');
  // Names stay for assistive technology while only icons are drawn.
  expect(within(nav).getByRole('link', { name: 'Home' })).toBeInTheDocument();
  expect(window.localStorage.getItem(`orcon.sidebar.collapsed.${currentUser.id}`)).toBe('true');
  first.unmount();

  // A later visit starts the way the person left it.
  await open('/orgs/acme/overview');
  expect(screen.getAllByRole('navigation', { name: 'Main navigation' })[0]).toHaveAttribute('data-collapsed', 'true');
});

it('the key N starts a flow in the project being looked at', async () => {
  const { router } = await open('/orgs/acme/projects/soporte/flows');
  await userEvent.keyboard('n');
  await waitFor(() => {
    expect(router.state.location.search).toEqual({ create: true });
  });
  expect(router.state.location.pathname).toBe('/orgs/acme/projects/soporte/flows');
  expect(await screen.findByRole('dialog')).toBeVisible();
});

it('the key N is left to a field while a person types', async () => {
  const { router } = await open('/orgs/acme/projects/ventas/flows');
  await userEvent.keyboard('n');
  const dialog = await screen.findByRole('dialog');
  const name = within(dialog).getByLabelText(/^Flow name/);
  await userEvent.clear(name);
  await userEvent.type(name, 'new');
  expect(name).toHaveValue('new');
  expect(router.state.location.pathname).toBe('/orgs/acme/projects/ventas/flows');
});

it('the question mark lists the keyboard shortcuts', async () => {
  await open('/orgs/acme/overview');
  await userEvent.keyboard('?');
  const dialog = await screen.findByRole('dialog', { name: 'Keyboard shortcuts' });
  expect(within(dialog).getByText('Create a flow')).toBeVisible();
  expect(within(dialog).getByText('Collapse or expand the sidebar')).toBeVisible();
});
