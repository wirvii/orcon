// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiCall, apiRequest } from '../../../app/api/http';
import { CredentialNameTaken, CredentialNotFound, type CredentialStore } from '../../../app/credential-store/store';
import { problemStatus } from '../../../app/errors/to-problem-details';

/** A credential as the API answers it: the fields the screens show, never a secret value (SPEC-004 R-CRED-01). */
const summarySchema = z.looseObject({
  id: z.string().min(1),
  name: z.string(),
  type: z.string(),
  updatedAt: z.string(),
});

const pageSchema = z.object({ items: z.array(summarySchema), nextPageToken: z.string().max(1024).optional() });

/**
 * The credential store over the API (SPEC-005: `projects/{projectId}/credentials` and
 * `credentials/{credentialId}`). These operations are not in `packages/openapi/v1` yet, so the paths
 * follow the names of the spec and the tests answer them with MSW. Secret values are sent once, in the
 * body of a create or a replacement, and no response carries them.
 */
export function createServerCredentialStore(): CredentialStore {
  return {
    list: async (projectId) => {
      const page = await apiRequest({
        method: 'GET',
        path: `/projects/${encodeURIComponent(projectId)}/credentials`,
        query: { limit: 200 },
        schema: pageSchema,
        source: 'response of GET /projects/{projectId}/credentials',
      });
      return page.items;
    },
    create: (projectId, input) =>
      apiRequest({
        method: 'POST',
        path: `/projects/${encodeURIComponent(projectId)}/credentials`,
        body: { name: input.name, type: input.type, fields: input.fields },
        schema: summarySchema,
        source: 'response of POST /projects/{projectId}/credentials',
      }).catch((error: unknown) => {
        if (problemStatus(error) === 409) throw new CredentialNameTaken(input.name);
        throw error;
      }),
    rotate: (credentialId, fields) =>
      apiRequest({
        method: 'PATCH',
        path: `/credentials/${encodeURIComponent(credentialId)}`,
        body: { fields },
        schema: summarySchema,
        source: 'response of PATCH /credentials/{credentialId}',
      }).catch((error: unknown) => {
        if (problemStatus(error) === 404) throw new CredentialNotFound(credentialId);
        throw error;
      }),
    remove: (credentialId) =>
      apiCall({ method: 'DELETE', path: `/credentials/${encodeURIComponent(credentialId)}`, source: 'response of DELETE /credentials/{credentialId}' })
        .then(() => undefined)
        .catch((error: unknown) => {
          if (problemStatus(error) === 404) throw new CredentialNotFound(credentialId);
          throw error;
        }),
  };
}
