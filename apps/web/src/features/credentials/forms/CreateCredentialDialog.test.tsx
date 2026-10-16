// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it, vi } from 'vitest';

import { project } from '../../../../test/product-fixtures';
import { renderInApp, server } from '../../../../test/screen-harness';
import { acme, problem } from '../../../../test/session-fixtures';
import { credentialApi, httpNodeType, siigo } from '../api/fixtures';

import { CreateCredentialDialog, type CreateCredentialDialogProps } from './CreateCredentialDialog';

function open(props: Partial<CreateCredentialDialogProps> = {}, ...handlers: Parameters<typeof server.use>) {
  server.use(...handlers, credentialApi.nodeTypes([httpNodeType]));
  const onOpenChange = vi.fn();
  renderInApp(<CreateCredentialDialog open onOpenChange={onOpenChange} orgId={acme.id} projectId={project.id} projectName="Ventas" {...props} />);
  return { onOpenChange };
}

const dialog = () => screen.findByRole('dialog', { name: 'Create credential' });

describe('CreateCredentialDialog', () => {
  it('lists services with brand icons and search', async () => {
    open();
    const box = await dialog();
    const services = within(box).getByRole('list', { name: 'Service' });
    expect(
      within(services)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['API key in a header', 'Bearer token', 'Username and password']);
    // The icon of the HTTP connector comes from the catalog of node types; before it arrives, the plug glyph stands in.
    await waitFor(() => {
      expect(services.querySelector('img')).toHaveAttribute('src', expect.stringContaining('data:image/svg+xml'));
    });

    const search = within(box).getByRole('searchbox', {
      name: 'Search service',
    });
    await userEvent.type(search, 'bearer');
    expect(
      within(services)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['Bearer token']);
    await userEvent.clear(search);
    await userEvent.type(search, 'zzz');
    expect(within(box).getByText('No service matches “zzz”.')).toBeVisible();
  });

  it('shows the connection guide next to the form', async () => {
    open();
    const box = await dialog();
    const guide = within(box).getByRole('complementary', {
      name: 'How to get these details',
    });
    expect(within(guide).getAllByRole('listitem')).toHaveLength(4);
    expect(within(guide).getByText(/The same as an n8n Header Auth credential/)).toBeVisible();
    expect(within(guide).getByText(/stops a badly built flow from leaking your key/)).toBeVisible();
    // The form is in the same dialog, so nobody leaves for the documentation.
    expect(within(box).getByLabelText(/Header name/)).toHaveValue('X-API-Key');

    await userEvent.click(within(box).getByRole('button', { name: 'Bearer token' }));
    expect(within(guide).getByText(/The same as an n8n Bearer Auth credential/)).toBeVisible();
    expect(within(box).getByLabelText(/^Token/)).toBeVisible();
    expect(within(box).queryByLabelText(/Header name/)).toBeNull();
  });

  it('secret fields are write only', async () => {
    open();
    const box = await dialog();
    const key = within(box).getByLabelText(/^Key/);
    expect(key).toHaveAttribute('type', 'password');
    expect(key).toHaveAttribute('autocomplete', 'new-password');
    expect(key).toHaveValue('');
    expect(within(box).getByText('Encrypted')).toBeVisible();
    // A non-secret field stays plain text.
    expect(within(box).getByLabelText(/Header name/)).toHaveAttribute('type', 'text');
  });

  it('HTTP header requires allowed domains', async () => {
    let called = false;
    open(
      {},
      http.post(`/api/v1/projects/${project.id}/credentials`, () => {
        called = true;
        return HttpResponse.json(siigo, { status: 201 });
      }),
    );
    const box = await dialog();
    await userEvent.type(within(box).getByLabelText(/^Credential name/), 'Siigo API');
    await userEvent.type(within(box).getByLabelText(/^Key/), 'secret-value');
    await userEvent.click(within(box).getByRole('button', { name: 'Create credential' }));
    expect(await within(box).findByText(/Add at least one domain/)).toBeVisible();
    expect(called).toBe(false);

    // Something that is not a domain stays in the box with the reason.
    const hosts = within(box).getByLabelText(/Send only to these domains/);
    await userEvent.type(hosts, 'not a domain{Enter}');
    expect(await within(box).findByText(/That is not a domain/)).toBeVisible();
    expect(called).toBe(false);
  });

  it('hides test connection without the operation', async () => {
    open();
    const box = await dialog();
    expect(within(box).queryByRole('button', { name: /test connection/i })).toBeNull();
  });

  it('shows test connection when the operation exists', async () => {
    const onTestConnection = vi.fn();
    open({ onTestConnection });
    const box = await dialog();
    await userEvent.click(within(box).getByRole('button', { name: 'Test connection' }));
    expect(onTestConnection).toHaveBeenCalledOnce();
  });

  it('sends the secret apart from the public fields, with the domains, and closes', async () => {
    let body: unknown;
    const { onOpenChange } = open(
      {},
      http.post(`/api/v1/projects/${project.id}/credentials`, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(siigo, {
          status: 201,
          headers: { ETag: '"1-1"' },
        });
      }),
    );
    const box = await dialog();
    await userEvent.type(within(box).getByLabelText(/^Credential name/), '  Siigo API ');
    await userEvent.type(within(box).getByLabelText(/^Key/), 'secret-value');
    // A pasted address becomes its host; Enter adds it without submitting the form.
    await userEvent.type(within(box).getByLabelText(/Send only to these domains/), 'https://API.siigo.com/v1/x{Enter}');
    const added = within(box).getByRole('list', { name: 'Allowed domains' });
    expect(within(added).getByText('api.siigo.com')).toBeVisible();
    await userEvent.type(within(box).getByLabelText(/Send only to these domains/), '*.siigo.com');
    await userEvent.click(within(box).getByRole('button', { name: 'Create credential' }));
    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
    expect(body).toEqual({
      name: 'Siigo API',
      type: 'orcon.http@1',
      allowedHosts: ['api.siigo.com', '*.siigo.com'],
      publicFields: { headerName: 'X-API-Key' },
      secretFields: { apiKey: 'secret-value' },
    });
  });

  it('says so under the name when it is taken', async () => {
    open(
      {},
      http.post(`/api/v1/projects/${project.id}/credentials`, () => problem(409, 'Conflict', 'conflict')),
    );
    const box = await dialog();
    await userEvent.type(within(box).getByLabelText(/^Credential name/), 'Siigo API');
    await userEvent.type(within(box).getByLabelText(/^Key/), 'secret-value');
    await userEvent.type(within(box).getByLabelText(/Send only to these domains/), 'api.siigo.com{Enter}');
    await userEvent.click(within(box).getByRole('button', { name: 'Create credential' }));
    expect(await within(box).findByText('A credential with this name already exists in the project.')).toBeVisible();
  });

  it('keeps the plug glyph when the catalog of node types is not available', async () => {
    open({}, credentialApi.nodeTypesFail());
    const box = await dialog();
    expect(within(box).getByRole('list', { name: 'Service' })).toBeVisible();
    await waitFor(() => {
      expect(box.querySelectorAll('[data-connector-fallback]').length).toBeGreaterThan(0);
    });
  });
});
