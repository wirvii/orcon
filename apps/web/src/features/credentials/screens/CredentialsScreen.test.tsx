// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';

import { credentialApi, httpNodeType, modelCredential, shared, siigo, siigoDetail, whatsappCredential, whatsappNodeType } from '../../../app/credentials/api/fixtures';

const address = '/orgs/acme/projects/ventas/credentials';

function open(...handlers: Parameters<typeof server.use>) {
  // The first handler that matches answers, so each test's own come before the default empty catalog of node types.
  server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]), ...handlers, credentialApi.nodeTypes([]));
  return renderRoute(address);
}

const rowOf = async (name: string) => (await screen.findByText(name)).closest('li') as HTMLElement;

describe('CredentialsScreen', () => {
  it('loading', async () => {
    open(credentialApi.listWith(() => new Promise(() => undefined)));
    expect(await screen.findByRole('status', { name: 'Loading' })).toBeVisible();
    expect(screen.getByRole('heading', { level: 2, name: 'Credentials' })).toBeVisible();
  });

  it('empty', async () => {
    open(credentialApi.list([]));
    expect(await screen.findByText('No credentials yet')).toBeVisible();
    // The next step is offered twice: in the header and in the empty state.
    expect(screen.getAllByRole('button', { name: 'Create credential' })).toHaveLength(2);
  });

  it('error', async () => {
    open(credentialApi.listWith(() => problem(500, 'internal', 'internal')));
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
  });

  it('forbidden', async () => {
    open(credentialApi.listWith(() => problem(403, 'forbidden', 'forbidden')));
    expect(await screen.findByText('You do not have permission to see this')).toBeVisible();
  });

  it('data', async () => {
    open(credentialApi.list([siigo, shared]), credentialApi.nodeTypes([httpNodeType]));
    const row = await rowOf('Siigo API');
    expect(within(row).getByText('HTTP request')).toBeVisible();
    // The first allowed domain, and how many more there are.
    expect(within(row).getByText('api.siigo.com')).toBeVisible();
    expect(within(row).getByText('+1 more')).toBeVisible();
    expect(within(row).getByText('Secret set')).toBeVisible();
    // The brand icon of the connector, from the catalog of node types.
    await waitFor(() => {
      expect(within(row).getByRole('img', { name: 'HTTP request' })).toHaveAttribute('src', expect.stringContaining('data:image/svg+xml'));
    });
    // Nothing on the screen can show a stored value: a secret is only written, never read back.
    expect(document.querySelectorAll('input')).toHaveLength(0);
  });

  it('shows coming soon when the operation answers 503', async () => {
    open(credentialApi.listWith(() => problem(503, 'Service Unavailable', 'service-unavailable')));
    expect(
      await screen.findByRole('heading', {
        name: 'Credentials will be available soon',
      }),
    ).toBeVisible();
    expect(document.querySelector('[data-coming-soon]')).not.toBeNull();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('hides health usage and expiry without data', async () => {
    open(credentialApi.list([siigo]));
    await rowOf('Siigo API');
    // None of these exist in the contract yet: no column, no empty cell, no invented value.
    expect(screen.queryByText(/used in/i)).toBeNull();
    expect(screen.queryByText(/last used/i)).toBeNull();
    expect(screen.queryByText(/expire/i)).toBeNull();
    expect(screen.queryByText(/connected|healthy|failed its test/i)).toBeNull();
    expect(screen.queryAllByRole('columnheader')).toHaveLength(0);
  });

  it('names the WhatsApp and the AI model credentials by their service, with the icon of the package', async () => {
    open(credentialApi.list([whatsappCredential, modelCredential]), credentialApi.nodeTypes([httpNodeType, whatsappNodeType]));
    const whatsapp = await rowOf('Ventas WhatsApp');
    expect(within(whatsapp).getByText('WhatsApp Cloud API')).toBeVisible();
    expect(within(whatsapp).getByText('graph.facebook.com')).toBeVisible();
    await waitFor(() => {
      expect(within(whatsapp).getByRole('img', { name: 'WhatsApp Cloud API' })).toHaveAttribute('src', expect.stringContaining('data:image/svg+xml'));
    });
    const model = await rowOf('Agent model key');
    expect(within(model).getByText('AI model API key')).toBeVisible();
  });

  it('replaces the secret of a WhatsApp credential and leaves out what was not typed', async () => {
    let body: unknown;
    const detail = { ...whatsappCredential };
    open(
      credentialApi.list([whatsappCredential]),
      credentialApi.get(whatsappCredential, '"2-1"', detail),
      http.put(`/api/v1/credentials/${whatsappCredential.id}/secret`, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ ...whatsappCredential, versionCount: 3 }, { headers: { ETag: '"3-1"' } });
      }),
    );
    const row = await rowOf('Ventas WhatsApp');
    await userEvent.click(within(row).getByRole('button', { name: 'Actions for Ventas WhatsApp' }));
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Replace secret' }));
    const dialog = await screen.findByRole('dialog', { name: 'Replace the secret of Ventas WhatsApp' });
    // A version holds all the values: the dialog says so, and only the access token is required.
    expect(await within(dialog).findByText(/replaces all the secret values/)).toBeVisible();
    await userEvent.type(within(dialog).getByLabelText(/^Access token/), 'new-token');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Replace secret' }));
    await waitFor(() => {
      expect(body).toEqual({ secretFields: { token: 'new-token' } });
    });
  });

  it('marks a shared credential and gives it no actions', async () => {
    open(credentialApi.list([siigo, shared]));
    const row = await rowOf('Shared Slack token');
    expect(within(row).getByText('Shared with this project')).toBeVisible();
    expect(within(row).queryByRole('button')).toBeNull();
    const own = await rowOf('Siigo API');
    expect(within(own).getByRole('button', { name: 'Actions for Siigo API' })).toBeVisible();
  });

  it('replaces the secret with the entity tag it read, and shows no old value', async () => {
    let seen: { ifMatch: string | null; body: unknown } | undefined;
    open(
      credentialApi.list([siigo]),
      credentialApi.get(siigo, '"2-1"', siigoDetail),
      http.put(`/api/v1/credentials/${siigo.id}/secret`, async ({ request }) => {
        seen = {
          ifMatch: request.headers.get('If-Match'),
          body: await request.json(),
        };
        return HttpResponse.json({ ...siigo, versionCount: 3 }, { headers: { ETag: '"3-1"' } });
      }),
    );
    const row = await rowOf('Siigo API');
    await userEvent.click(within(row).getByRole('button', { name: 'Actions for Siigo API' }));
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Replace secret' }));
    const dialog = await screen.findByRole('dialog', {
      name: 'Replace the secret of Siigo API',
    });
    const key = await within(dialog).findByLabelText(/^Key/);
    expect(key).toHaveValue('');
    expect(key).toHaveAttribute('type', 'password');
    await userEvent.type(key, 'new-key-value');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Replace secret' }));
    await waitFor(() => {
      expect(
        screen.queryByRole('dialog', {
          name: 'Replace the secret of Siigo API',
        }),
      ).toBeNull();
    });
    expect(seen).toEqual({
      ifMatch: '"2-1"',
      body: {
        secretFields: { apiKey: 'new-key-value' },
        publicFields: { headerName: 'X-API-Key' },
      },
    });
  });

  it('says so when somebody else changed the credential first', async () => {
    open(
      credentialApi.list([siigo]),
      credentialApi.get(siigo, '"2-1"', siigoDetail),
      http.patch(`/api/v1/credentials/${siigo.id}`, () => problem(412, 'Precondition Failed', 'precondition-failed')),
    );
    const row = await rowOf('Siigo API');
    await userEvent.click(within(row).getByRole('button', { name: 'Actions for Siigo API' }));
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Change allowed domains' }));
    const dialog = await screen.findByRole('dialog', {
      name: 'Allowed domains of Siigo API',
    });
    await within(dialog).findByRole('list', { name: 'Allowed domains' });
    await userEvent.type(within(dialog).getByLabelText(/Send only to these domains/), 'extra.siigo.com{Enter}');
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save domains' }));
    expect(await within(dialog).findByText(/Someone else changed this credential/)).toBeVisible();
  });

  it('changes the allowed domains', async () => {
    let body: unknown;
    open(
      credentialApi.list([siigo]),
      credentialApi.get(siigo, '"2-1"', siigoDetail),
      http.patch(`/api/v1/credentials/${siigo.id}`, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ ...siigo, allowedHosts: ['api.siigo.com'] }, { headers: { ETag: '"2-2"' } });
      }),
    );
    const row = await rowOf('Siigo API');
    await userEvent.click(within(row).getByRole('button', { name: 'Actions for Siigo API' }));
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Change allowed domains' }));
    const dialog = await screen.findByRole('dialog', {
      name: 'Allowed domains of Siigo API',
    });
    await userEvent.click(await within(dialog).findByRole('button', { name: 'Remove *.siigo.com' }));
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save domains' }));
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: 'Allowed domains of Siigo API' })).toBeNull();
    });
    expect(body).toEqual({ allowedHosts: ['api.siigo.com'] });
  });

  it('asks before it deletes a credential, and deletes it with its entity tag', async () => {
    let ifMatch: string | null = null;
    open(
      credentialApi.list([siigo]),
      credentialApi.get(siigo, '"2-1"', siigoDetail),
      http.delete(`/api/v1/credentials/${siigo.id}`, ({ request }) => {
        ifMatch = request.headers.get('If-Match');
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const row = await rowOf('Siigo API');
    await userEvent.click(within(row).getByRole('button', { name: 'Actions for Siigo API' }));
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Delete' }));
    const question = await screen.findByRole('alertdialog', {
      name: 'Delete Siigo API?',
    });
    expect(within(question).getByText(/cannot be undone/)).toBeVisible();
    await userEvent.click(within(question).getByRole('button', { name: 'Delete credential' }));
    await waitFor(() => {
      expect(ifMatch).toBe('"2-1"');
    });
  });
});
