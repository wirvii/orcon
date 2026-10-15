// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { flow, listedFlow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, signedIn } from '../../../../test/session-fixtures';

const address = '/orgs/acme/projects/ventas/flows';
const newFlowId = '0195f1a0-0000-7000-8000-0000000000e2';

const beforeChooser = () => [signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]), productApi.orgFlows([listedFlow])];

async function openChooser(path = address) {
  server.use(...beforeChooser());
  const view = renderRoute(path);
  // A chooser that opens on arrival hides the page behind it from assistive technology, so wait for it instead.
  if (path.includes('create')) await screen.findByRole('dialog', { name: 'Create flow' });
  else await screen.findByRole('link', { name: 'Order notifications' });
  return view;
}

describe('CreateFlowChooser', () => {
  it('opens with N and offers four options', async () => {
    const { router } = await openChooser();
    await userEvent.keyboard('n');
    const dialog = await screen.findByRole('dialog', { name: 'Create flow' });
    // The four ways to start, each with its key.
    const options = within(within(dialog).getByRole('group', { name: 'How to start' })).getAllByRole('button');
    expect(options.map((option) => option.getAttribute('aria-keyshortcuts'))).toEqual(['B', 'T', 'I', 'A']);
    expect(within(dialog).getByRole('button', { name: /Blank/ })).toHaveAttribute('aria-pressed', 'true');
    // The project is chosen in the footer, and the address asks for the chooser until it closes.
    expect(within(dialog).getByRole('combobox')).toHaveTextContent('Ventas');
    expect(router.state.location.search).toMatchObject({ create: true });
    await userEvent.keyboard('{Escape}');
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: 'Create flow' })).toBeNull();
    });
    expect(router.state.location.search).not.toHaveProperty('create');
  });

  it('selects options with B T and I', async () => {
    await openChooser(`${address}?create=true`);
    const dialog = await screen.findByRole('dialog', { name: 'Create flow' });
    await userEvent.keyboard('t');
    expect(within(dialog).getByRole('button', { name: /From a template/ })).toHaveAttribute('aria-pressed', 'true');
    await userEvent.keyboard('i');
    expect(within(dialog).getByRole('button', { name: /Import from n8n/ })).toHaveAttribute('aria-pressed', 'true');
    await userEvent.keyboard('b');
    expect(within(dialog).getByRole('button', { name: /Blank/ })).toHaveAttribute('aria-pressed', 'true');
    // Typing a name never switches the option.
    await userEvent.clear(within(dialog).getByLabelText(/^Flow name/));
    await userEvent.type(within(dialog).getByLabelText(/^Flow name/), 'tib');
    expect(within(dialog).getByRole('button', { name: /Blank/ })).toHaveAttribute('aria-pressed', 'true');
    expect(within(dialog).getByLabelText(/^Flow name/)).toHaveValue('tib');
  });

  it('blank creates an inactive flow and opens the editor', async () => {
    let body: unknown;
    server.use(
      productApi.createFlow(async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ ...flow, id: newFlowId, name: 'Refunds' }, { status: 201 });
      }),
    );
    const { router } = await openChooser(`${address}?create=true`);
    const dialog = await screen.findByRole('dialog', { name: 'Create flow' });
    await userEvent.clear(within(dialog).getByLabelText(/^Flow name/));
    await userEvent.type(within(dialog).getByLabelText(/^Flow name/), 'Refunds');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Create and open' }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/orgs/acme/projects/ventas/flows/${newFlowId}`);
    });
    // Only the name is sent: the server creates the flow inactive with an empty draft.
    expect(body).toEqual({ name: 'Refunds' });
  });

  it('template shows coming soon without sample cards', async () => {
    await openChooser(`${address}?create=template`);
    const dialog = await screen.findByRole('dialog', { name: 'Create flow' });
    expect(within(dialog).getByRole('button', { name: /From a template/ })).toHaveAttribute('aria-pressed', 'true');
    expect(within(dialog).getByRole('heading', { name: 'Templates' })).toBeVisible();
    expect(dialog.querySelector('[data-coming-soon]')).not.toBeNull();
    // No invented examples, no way to continue.
    expect(within(dialog).queryByRole('button', { name: 'Create and open' })).toBeNull();
    expect(within(dialog).getAllByRole('button').filter((button) => button.getAttribute('aria-keyshortcuts') === null && /template/i.test(button.textContent))).toHaveLength(0);
  });

  it('AI shows coming soon and never activates', async () => {
    await openChooser(`${address}?create=ai`);
    const dialog = await screen.findByRole('dialog', { name: 'Create flow' });
    expect(within(dialog).getByRole('heading', { name: 'Build with AI' })).toBeVisible();
    expect(dialog.querySelector('[data-coming-soon]')).not.toBeNull();
    expect(within(dialog).queryByRole('button', { name: 'Create and open' })).toBeNull();
    expect(within(dialog).queryByRole('textbox')).toBeNull();
  });

  it('import shows coming soon when the converter is unavailable', async () => {
    await openChooser(`${address}?create=import`);
    const dialog = await screen.findByRole('dialog', { name: 'Create flow' });
    expect(within(dialog).getByRole('heading', { name: 'Import from n8n' })).toBeVisible();
    expect(dialog.querySelector('[data-coming-soon]')).not.toBeNull();
    expect(within(dialog).queryByRole('button', { name: 'Create and open' })).toBeNull();
  });
});
