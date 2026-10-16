// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it, vi } from 'vitest';

import { project } from '../../../../test/product-fixtures';
import { renderInApp, server } from '../../../../test/screen-harness';
import { acme, problem } from '../../../../test/session-fixtures';
import { credentialApi, httpNodeType, modelCredential, siigo, whatsappCredential, whatsappNodeType } from '../api/fixtures';

import { SERVICES } from '../model/services';
import { CreateCredentialDialog, type CreateCredentialDialogProps } from './CreateCredentialDialog';

function open(props: Partial<CreateCredentialDialogProps> = {}, ...handlers: Parameters<typeof server.use>) {
  server.use(...handlers, credentialApi.nodeTypes([httpNodeType]));
  const onOpenChange = vi.fn();
  renderInApp(<CreateCredentialDialog open onOpenChange={onOpenChange} orgId={acme.id} projectId={project.id} projectName="Ventas" {...props} />);
  return { onOpenChange };
}

const dialog = () => screen.findByRole('dialog', { name: 'Create credential' });

/** The generic header service: the dialog opens on WhatsApp, the main use, so these tests choose the one they are about. */
const chooseHeader = (box: HTMLElement) => userEvent.click(within(box).getByRole('button', { name: 'API key in a header' }));

describe('CreateCredentialDialog', () => {
  it('lists services with brand icons and search', async () => {
    open();
    const box = await dialog();
    const services = within(box).getByRole('list', { name: 'Service' });
    expect(
      within(services)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['WhatsApp Cloud API', 'AI model API key', 'API key in a header', 'Bearer token', 'Username and password']);
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
    await chooseHeader(box);
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
    await chooseHeader(box);
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
    await chooseHeader(box);
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
    await chooseHeader(box);
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
    await chooseHeader(box);
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

  describe('WhatsApp Cloud API', () => {
    it('is the service the dialog opens on, with the real icon from the catalog', async () => {
      open({}, credentialApi.nodeTypes([whatsappNodeType]));
      const box = await dialog();
      expect(within(box).getByRole('button', { name: 'WhatsApp Cloud API' })).toHaveAttribute('aria-pressed', 'true');
      expect(within(box).getByRole('heading', { level: 3, name: 'WhatsApp Cloud API' })).toBeVisible();
      await waitFor(() => {
        expect(within(box).getByRole('button', { name: 'WhatsApp Cloud API' }).querySelector('img')).toHaveAttribute('src', expect.stringContaining('data:image/svg+xml'));
      });
    });

    it('asks for the fields the package reads and starts with Meta as the only domain', async () => {
      open();
      const box = await dialog();
      // The names are those of the package: `token` sends, `appSecret` and `verifyToken` check what comes in.
      for (const label of [/^Access token/, /^App secret/, /^Verify token/]) expect(within(box).getByLabelText(label)).toHaveAttribute('type', 'password');
      expect(within(box).getByLabelText(/^Access token/)).toBeRequired();
      const added = within(box).getByRole('list', { name: 'Allowed domains' });
      expect(within(added).getByText('graph.facebook.com')).toBeVisible();
    });

    it('shows the steps in the console of Meta and the note for people who come from n8n', async () => {
      open();
      const box = await dialog();
      const guide = within(box).getByRole('complementary', { name: 'How to get these details' });
      expect(within(guide).getAllByRole('listitem')).toHaveLength(6);
      expect(within(guide).getByText(/developers\.facebook\.com/)).toBeVisible();
      expect(within(guide).getByText(/App settings, Basic/)).toBeVisible();
      expect(within(guide).getByText(/Configuration, Webhook/)).toBeVisible();
      expect(within(guide).getByText(/In n8n the WhatsApp credential asks for an access token and a business account ID/)).toBeVisible();
    });

    it('sends the token and the two checks of the webhook as secrets, and leaves an empty optional one out', async () => {
      let body: unknown;
      const { onOpenChange } = open(
        {},
        http.post(`/api/v1/projects/${project.id}/credentials`, async ({ request }) => {
          body = await request.json();
          return HttpResponse.json(whatsappCredential, { status: 201, headers: { ETag: '"1-1"' } });
        }),
      );
      const box = await dialog();
      await userEvent.type(within(box).getByLabelText(/^Credential name/), 'Ventas WhatsApp');
      await userEvent.type(within(box).getByLabelText(/^Access token/), 'EAAB-token');
      await userEvent.type(within(box).getByLabelText(/^App secret/), 'app-secret');
      await userEvent.click(within(box).getByRole('button', { name: 'Create credential' }));
      await waitFor(() => {
        expect(onOpenChange).toHaveBeenCalledWith(false);
      });
      expect(body).toEqual({
        name: 'Ventas WhatsApp',
        type: 'orcon.whatsapp@1',
        allowedHosts: ['graph.facebook.com'],
        secretFields: { token: 'EAAB-token', appSecret: 'app-secret' },
      });
    });

    it('starts the domains again when another service is chosen and back', async () => {
      open();
      const box = await dialog();
      await userEvent.click(within(box).getByRole('button', { name: 'Bearer token' }));
      expect(within(box).queryByRole('list', { name: 'Allowed domains' })).toBeNull();
      await userEvent.click(within(box).getByRole('button', { name: 'WhatsApp Cloud API' }));
      expect(within(within(box).getByRole('list', { name: 'Allowed domains' })).getByText('graph.facebook.com')).toBeVisible();
    });
  });

  describe('AI model API key', () => {
    it('asks for one secret field and no domain until the person adds the one of the provider', async () => {
      open();
      const box = await dialog();
      await userEvent.click(within(box).getByRole('button', { name: 'AI model API key' }));
      expect(within(box).getByLabelText(/^API key/)).toHaveAttribute('type', 'password');
      expect(within(box).queryByRole('list', { name: 'Allowed domains' })).toBeNull();
      const guide = within(box).getByRole('complementary', { name: 'How to get these details' });
      expect(within(guide).getAllByRole('listitem')).toHaveLength(4);
      expect(within(guide).getByText(/The same as the API key credential of an n8n model node/)).toBeVisible();
    });

    it('creates the credential bound to the domain that was typed', async () => {
      let body: unknown;
      const { onOpenChange } = open(
        {},
        http.post(`/api/v1/projects/${project.id}/credentials`, async ({ request }) => {
          body = await request.json();
          return HttpResponse.json(modelCredential, { status: 201, headers: { ETag: '"1-1"' } });
        }),
      );
      const box = await dialog();
      await userEvent.click(within(box).getByRole('button', { name: 'AI model API key' }));
      await userEvent.type(within(box).getByLabelText(/^Credential name/), 'Agent model key');
      await userEvent.type(within(box).getByLabelText(/^API key/), 'sk-test');
      await userEvent.click(within(box).getByRole('button', { name: 'Create credential' }));
      expect(await within(box).findByText(/Add at least one domain/)).toBeVisible();
      await userEvent.type(within(box).getByLabelText(/Send only to these domains/), 'api.provider.example{Enter}');
      await userEvent.click(within(box).getByRole('button', { name: 'Create credential' }));
      await waitFor(() => {
        expect(onOpenChange).toHaveBeenCalledWith(false);
      });
      expect(body).toEqual({ name: 'Agent model key', type: 'orcon.ai.llm@1', allowedHosts: ['api.provider.example'], secretFields: { apiKey: 'sk-test' } });
    });
  });

  it('offers only the services that fit a step and hands back what the server created', async () => {
    const onCreated = vi.fn();
    const { onOpenChange } = open(
      { services: SERVICES.filter((service) => service.id === 'whatsapp'), onCreated, initialName: 'Sales line' },
      http.post(`/api/v1/projects/${project.id}/credentials`, () => HttpResponse.json(whatsappCredential, { status: 201, headers: { ETag: '"1-1"' } })),
    );
    const box = await dialog();
    // One service has nothing to choose between: no list, only its form.
    expect(within(box).queryByRole('list', { name: 'Service' })).toBeNull();
    expect(within(box).getByLabelText(/^Credential name/)).toHaveValue('Sales line');
    await userEvent.type(within(box).getByLabelText(/^Access token/), 'EAAB-token');
    await userEvent.click(within(box).getByRole('button', { name: 'Create credential' }));
    await waitFor(() => {
      expect(onCreated).toHaveBeenCalledWith(expect.objectContaining({ id: whatsappCredential.id, name: 'Ventas WhatsApp' }));
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
