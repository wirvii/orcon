// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { HttpResponse, http } from 'msw';
import { describe, expect, it } from 'vitest';

import { setActiveOrganization } from '../../../app/api/http';
import { CredentialNameTaken, CredentialNotFound } from '../../../app/credential-store/store';
import { server } from '../../../../test/screen-harness';
import { problem } from '../../../../test/session-fixtures';

import { createServerCredentialStore } from './server-credential-store';

const projectId = '0195f1a0-0000-7000-8000-0000000000d1';
const credentialId = '0195f1a0-0000-7000-8000-0000000000f1';
const tag = '"2-1"';
const summary = { id: credentialId, projectId, name: 'Support line', type: 'whatsappCloudApi', updatedAt: '2026-10-02T16:40:00Z', status: 'OK' };
const withTag = (body: Record<string, unknown>, status = 200) => HttpResponse.json(body, { status, headers: { ETag: tag } });

describe('the credential store over the API', () => {
  it('sends the active organization in Orcon-Org while one is open, and nothing once it is cleared', async () => {
    const seen: (string | null)[] = [];
    server.use(
      http.get(`/api/v1/projects/${projectId}/credentials`, ({ request }) => {
        seen.push(request.headers.get('Orcon-Org'));
        return HttpResponse.json({ items: [] });
      }),
    );
    setActiveOrganization('0195f1a0-0000-7000-8000-0000000000c1');
    await createServerCredentialStore().list(projectId);
    setActiveOrganization(undefined);
    await createServerCredentialStore().list(projectId);
    expect(seen).toEqual(['0195f1a0-0000-7000-8000-0000000000c1', null]);
  });

  it('lists the credentials of a project', async () => {
    server.use(http.get(`/api/v1/projects/${projectId}/credentials`, () => HttpResponse.json({ items: [summary] })));
    await expect(createServerCredentialStore().list(projectId)).resolves.toEqual([summary]);
  });

  it('sends the secret values and the public values in their own objects, once', async () => {
    let body: unknown;
    server.use(
      http.post(`/api/v1/projects/${projectId}/credentials`, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(summary, { status: 201 });
      }),
    );
    const created = await createServerCredentialStore().create(projectId, {
      name: 'Support line',
      type: 'whatsappCloudApi',
      fields: { accessToken: 'tok', phoneNumberId: '1', appSecret: 'app', verifyToken: 'ver' },
    });
    expect(created).toEqual(summary);
    expect(body).toEqual({
      name: 'Support line',
      type: 'whatsappCloudApi',
      secretFields: { accessToken: 'tok', appSecret: 'app', verifyToken: 'ver' },
      publicFields: { phoneNumberId: '1' },
    });
  });

  it('reports a name that is taken as CredentialNameTaken', async () => {
    server.use(http.post(`/api/v1/projects/${projectId}/credentials`, () => problem(409, 'conflict', 'conflict')));
    await expect(createServerCredentialStore().create(projectId, { name: 'Support line', type: 'modelProviderApiKey', fields: { apiKey: 'k' } })).rejects.toBeInstanceOf(
      CredentialNameTaken,
    );
  });

  it('replaces the secret with the entity tag it was read at, and reports a missing credential as CredentialNotFound', async () => {
    let ifMatch: string | null = null;
    let body: unknown;
    server.use(
      http.get(`/api/v1/credentials/${credentialId}`, () => withTag(summary)),
      http.put(`/api/v1/credentials/${credentialId}/secret`, async ({ request }) => {
        ifMatch = request.headers.get('If-Match');
        body = await request.json();
        return withTag(summary);
      }),
    );
    await expect(createServerCredentialStore().rotate(credentialId, { accessToken: 'new' })).resolves.toEqual(summary);
    expect(ifMatch).toBe(tag);
    expect(body).toEqual({ secretFields: { accessToken: 'new' } });

    server.use(http.get(`/api/v1/credentials/${credentialId}`, () => problem(404, 'not-found', 'not-found')));
    await expect(createServerCredentialStore().rotate(credentialId, { accessToken: 'new' })).rejects.toBeInstanceOf(CredentialNotFound);
  });

  it('deletes a credential with the entity tag it was read at', async () => {
    let ifMatch: string | null = null;
    server.use(
      http.get(`/api/v1/credentials/${credentialId}`, () => withTag(summary)),
      http.delete(`/api/v1/credentials/${credentialId}`, ({ request }) => {
        ifMatch = request.headers.get('If-Match');
        return new HttpResponse(null, { status: 204 });
      }),
    );
    await expect(createServerCredentialStore().remove(credentialId)).resolves.toBeUndefined();
    expect(ifMatch).toBe(tag);
  });
});
