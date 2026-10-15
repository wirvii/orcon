// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiCall, apiRequest } from '../../../app/api/http';
import { credentialType } from '../../../app/credential-store/credential-types';
import { CredentialNameTaken, CredentialNotFound, type CredentialStore, type NewCredential } from '../../../app/credential-store/store';
import { ContractError } from '../../../app/errors/contract-error';
import { problemStatus } from '../../../app/errors/to-problem-details';

/**
 * A credential as the API answers it (SPEC-005 `Credential`): the fields the screens show and the state of
 * its secret, never a secret value (SPEC-004 R-CRED-01). The loose object keeps the fields the screens do not read.
 */
const summarySchema = z.looseObject({
  id: z.string().min(1),
  name: z.string(),
  type: z.string(),
  updatedAt: z.string(),
});

const pageSchema = z.object({ items: z.array(summarySchema), nextPageToken: z.string().max(1024).optional() });

/**
 * Splits the values of a form into the secret and the public fields of its type: the API takes them in
 * two objects, and only the first one is write-only (SPEC-005 `CreateCredentialRequest`).
 */
function splitFields(type: string, fields: Readonly<Record<string, string>>) {
  const declared = credentialType(type)?.fields ?? [];
  const secretFields: Record<string, string> = {};
  const publicFields: Record<string, string> = {};
  for (const [name, value] of Object.entries(fields)) {
    if (declared.some((field) => field.name === name && field.secret)) secretFields[name] = value;
    else publicFields[name] = value;
  }
  return { secretFields, publicFields };
}

/**
 * The credential store over the API (SPEC-005 credentials, contract in PR #131): `GET|POST
 * /projects/{projectId}/credentials`, `GET|PATCH|DELETE /credentials/{credentialId}`, and
 * `PUT /credentials/{credentialId}/secret` for a replacement. The writes that change an existing credential
 * send the entity tag they were based on (`If-Match`), which the store reads just before it writes; a
 * credential changed by someone else answers 412 and nothing is overwritten. Secret values go out once, in
 * the body, and no response carries them.
 */
export function createServerCredentialStore(): CredentialStore {
  /** The entity tag of the credential as it is now: the `If-Match` of the change that follows. */
  const currentTag = async (credentialId: string, source: string): Promise<string> => {
    const { etag } = await apiCall({ method: 'GET', path: `/credentials/${encodeURIComponent(credentialId)}`, schema: summarySchema, source });
    if (etag === undefined) throw new ContractError(source, []);
    return etag;
  };

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
    create: (projectId, input: NewCredential) => {
      const { secretFields, publicFields } = splitFields(input.type, input.fields);
      return apiRequest({
        method: 'POST',
        path: `/projects/${encodeURIComponent(projectId)}/credentials`,
        body: { name: input.name, type: input.type, secretFields, ...(Object.keys(publicFields).length === 0 ? {} : { publicFields }) },
        schema: summarySchema,
        source: 'response of POST /projects/{projectId}/credentials',
      }).catch((error: unknown) => {
        if (problemStatus(error) === 409) throw new CredentialNameTaken(input.name);
        throw error;
      });
    },
    rotate: async (credentialId, fields) => {
      const source = 'response of PUT /credentials/{credentialId}/secret';
      try {
        const ifMatch = await currentTag(credentialId, 'response of GET /credentials/{credentialId}');
        const { value } = await apiCall({
          method: 'PUT',
          path: `/credentials/${encodeURIComponent(credentialId)}/secret`,
          body: { secretFields: fields },
          ifMatch,
          schema: summarySchema,
          source,
        });
        return value;
      } catch (error: unknown) {
        if (problemStatus(error) === 404) throw new CredentialNotFound(credentialId);
        throw error;
      }
    },
    remove: async (credentialId) => {
      try {
        const ifMatch = await currentTag(credentialId, 'response of GET /credentials/{credentialId}');
        await apiCall({ method: 'DELETE', path: `/credentials/${encodeURIComponent(credentialId)}`, ifMatch, source: 'response of DELETE /credentials/{credentialId}' });
      } catch (error: unknown) {
        if (problemStatus(error) === 404) throw new CredentialNotFound(credentialId);
        throw error;
      }
    },
  };
}
