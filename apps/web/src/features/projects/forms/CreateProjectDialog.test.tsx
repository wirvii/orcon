// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, signedIn } from '../../../../test/session-fixtures';

/** Opens the list of an organization with no projects and the dialog that creates one. */
async function openDialog(createProject: Parameters<typeof productApi.createProject>[0]) {
  server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([]), productApi.createProject(createProject));
  const view = renderRoute('/orgs/acme/projects');
  await screen.findByRole('heading', { name: 'No projects yet' });
  await userEvent.click(screen.getAllByRole('button', { name: 'New project' })[0] as HTMLElement);
  await screen.findByRole('dialog', { name: 'New project' });
  return view;
}

describe('CreateProjectDialog', () => {
  it('maps a validation error to the name field', async () => {
    await openDialog(() =>
      HttpResponse.json(
        {
          type: 'about:blank',
          status: 400,
          title: 'Validation failed',
          code: 'validation-failed',
          errors: [{ pointer: '/name', code: 'too-long', message: 'Use at most 120 characters.' }],
        },
        { status: 400, headers: { 'Content-Type': 'application/problem+json' } },
      ),
    );
    await userEvent.type(screen.getByLabelText(/^Name/), 'Ventas');
    await userEvent.click(screen.getByRole('button', { name: 'Create project' }));
    expect(await screen.findByText('Use at most 120 characters.')).toBeVisible();
    expect(screen.getByLabelText(/^Name/)).toHaveAttribute('aria-invalid', 'true');
  });

  it('shows a generic message on conflict', async () => {
    await openDialog(() => HttpResponse.json({ status: 409, title: 'Conflict', code: 'conflict' }, { status: 409, headers: { 'Content-Type': 'application/problem+json' } }));
    await userEvent.type(screen.getByLabelText(/^Name/), 'Ventas');
    await userEvent.click(screen.getByRole('button', { name: 'Create project' }));
    expect(await screen.findByText('This name is not available. Choose another name.')).toBeVisible();
  });

  it('creates the project and opens its flows', async () => {
    await openDialog(() => HttpResponse.json(project, { status: 201 }));
    // From now on the project exists: the flows address resolves its handle.
    server.use(productApi.projects([project]), productApi.orgFlows([]));
    await userEvent.type(screen.getByLabelText(/^Name/), 'Ventas');
    await userEvent.click(screen.getByRole('button', { name: 'Create project' }));
    expect(await screen.findByRole('heading', { name: 'Ventas is ready for its first flow' })).toBeVisible();
  });
});
