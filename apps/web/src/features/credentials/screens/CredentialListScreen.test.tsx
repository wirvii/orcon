// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { project, productApi } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, signedIn } from '../../../../test/session-fixtures';

const address = '/orgs/acme/projects/ventas/credentials';

function openList() {
  server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]));
  return renderRoute(address);
}

describe('the credentials of a project', () => {
  it('lists the names and types of the credentials, and never a secret value', async () => {
    openList();
    const row = await screen.findByText('Support line');
    const item = row.closest('li');
    expect(item).not.toBeNull();
    expect(within(item as HTMLElement).getByText(/WhatsApp Cloud API/)).toBeVisible();
    expect(within(item as HTMLElement).getByText(/Secret set/)).toBeVisible();
    // The screen has no input that shows a stored value: a secret is only written, never read back.
    expect(document.querySelectorAll('input[value]:not([type="hidden"])')).toHaveLength(0);
  });

  it('creates a credential and lists it, and says so when the name is taken', async () => {
    openList();
    await screen.findByText('Support line');
    await userEvent.click(screen.getAllByRole('button', { name: 'New credential' })[0] as HTMLElement);
    const dialog = await screen.findByRole('dialog', { name: 'New credential' });
    await userEvent.type(within(dialog).getByLabelText(/^Name/), 'Shop key');
    await userEvent.type(within(dialog).getByLabelText(/Header value/), 'secret-value');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Create credential' }));
    expect(await screen.findByText('Shop key')).toBeVisible();
    expect(screen.queryByRole('dialog', { name: 'New credential' })).toBeNull();
  });

  it('replaces the secret of a credential and keeps its name', async () => {
    openList();
    const item = (await screen.findByText('Slack incoming webhook')).closest('li') as HTMLElement;
    await userEvent.click(within(item).getByRole('button', { name: 'Replace secret' }));
    const dialog = await screen.findByRole('dialog', { name: 'Replace the secret of Slack incoming webhook' });
    await userEvent.type(within(dialog).getByLabelText(/Header value/), 'new-value');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Replace secret' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: 'Replace the secret of Slack incoming webhook' })).toBeNull();
    });
    expect(screen.getByText('Slack incoming webhook')).toBeVisible();
  });

  it('asks before it deletes a credential, and deletes it once the person confirms', async () => {
    openList();
    const item = (await screen.findByText('Model key production')).closest('li') as HTMLElement;
    await userEvent.click(within(item).getByRole('button', { name: 'Delete' }));
    const question = await screen.findByRole('alertdialog', { name: 'Delete Model key production?' });
    expect(within(question).getByText(/cannot be undone/)).toBeVisible();
    await userEvent.click(within(question).getByRole('button', { name: 'Delete credential' }));
    await waitFor(() => {
      expect(screen.queryByText('Model key production')).toBeNull();
    });
  });
});
