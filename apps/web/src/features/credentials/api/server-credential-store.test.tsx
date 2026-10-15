// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { HttpResponse, http } from 'msw';
import { describe, expect, it } from 'vitest';

import { CredentialNameTaken, CredentialNotFound } from '../../../app/credential-store/store';
import { server } from '../../../../test/screen-harness';
import { problem } from '../../../../test/session-fixtures';

import { createServerCredentialStore } from './server-credential-store';

const projectId = '0195f1a0-0000-7000-8000-0000000000d1';
const credentialId = '0195f1a0-0000-7000-8000-0000000000f1';
const summary = { id: credentialId, name: 'Support line', type: 'whatsappCloudApi', updatedAt: '2026-10-02T16:40:00Z' };

describe('the credential store over the API', () => {
  it('lists the credentials of a project from the planned route', async () => {
    server.use(http.get(`/api/v1/projects/${projectId}/credentials`, () => HttpResponse.json({ items: [summary] })));
    await expect(createServerCredentialStore().list(projectId)).resolves.toEqual([summary]);
  });

  it('sends the values of a new credential once and answers the summary', async () => {
    let body: unknown;
    server.use(
      http.post(`/api/v1/projects/${projectId}/credentials`, async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(summary, { status: 201 });
      }),
    );
    const created = await createServerCredentialStore().create(projectId, { name: 'Support line', type: 'whatsappCloudApi', fields: { accessToken: 'tok' } });
    expect(created).toEqual(summary);
    expect(body).toEqual({ name: 'Support line', type: 'whatsappCloudApi', fields: { accessToken: 'tok' } });
  });

  it('reports a name that is taken as CredentialNameTaken', async () => {
    server.use(http.post(`/api/v1/projects/${projectId}/credentials`, () => problem(409, 'conflict', 'conflict')));
    await expect(createServerCredentialStore().create(projectId, { name: 'Support line', type: 'modelProviderApiKey', fields: { apiKey: 'k' } })).rejects.toBeInstanceOf(
      CredentialNameTaken,
    );
  });

  it('replaces the secret of a credential with PATCH and reports a missing one as CredentialNotFound', async () => {
    server.use(http.patch(`/api/v1/credentials/${credentialId}`, () => HttpResponse.json(summary)));
    await expect(createServerCredentialStore().rotate(credentialId, { accessToken: 'new' })).resolves.toEqual(summary);
    server.use(http.patch(`/api/v1/credentials/${credentialId}`, () => problem(404, 'not-found', 'not-found')));
    await expect(createServerCredentialStore().rotate(credentialId, { accessToken: 'new' })).rejects.toBeInstanceOf(CredentialNotFound);
  });

  it('deletes a credential with DELETE and resolves without a body', async () => {
    let method = '';
    server.use(
      http.delete(`/api/v1/credentials/${credentialId}`, ({ request }) => {
        method = request.method;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    await expect(createServerCredentialStore().remove(credentialId)).resolves.toBeUndefined();
    expect(method).toBe('DELETE');
  });
});
