// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { apiCall, apiRequest } from '../../../app/api/http';
import { ContractError } from '../../../app/errors/contract-error';

import type { ServiceIcons } from '../model/service-icons';
import { credentialPageSchema, credentialSchema, nodeTypePageSchema, type Credential, type CredentialWithTag, type NewCredential } from './schema';

/** The most pages the list follows: 20 pages of 200 is far more credentials than a project has. */
const MAX_PAGES = 20;

/** The credentials the project can use, its own and the ones shared with it (`listCredentials`). */
export async function listCredentials(projectId: string): Promise<readonly Credential[]> {
  const items: Credential[] = [];
  let pageToken: string | undefined;
  for (let page = 0; page < MAX_PAGES; page += 1) {
    const answer = await apiRequest({
      method: 'GET',
      path: `/projects/${encodeURIComponent(projectId)}/credentials`,
      query: { limit: 200, pageToken },
      schema: credentialPageSchema,
      source: 'response of GET /projects/{projectId}/credentials',
    });
    items.push(...answer.items);
    pageToken = answer.nextPageToken;
    if (pageToken === undefined) break;
  }
  return items;
}

/** One credential with its public fields and the entity tag every change to it must send (`getCredential`). */
export async function getCredential(credentialId: string): Promise<CredentialWithTag> {
  const source = 'response of GET /credentials/{credentialId}';
  const { value, etag } = await apiCall({
    method: 'GET',
    path: `/credentials/${encodeURIComponent(credentialId)}`,
    schema: credentialSchema,
    source,
  });
  if (etag === undefined) throw new ContractError(source, []);
  return { credential: value, etag };
}

/** Creates a credential (`createCredential`). The secret goes out once, in the body, and no response carries it. */
export function createCredential(projectId: string, input: NewCredential): Promise<Credential> {
  return apiRequest({
    method: 'POST',
    path: `/projects/${encodeURIComponent(projectId)}/credentials`,
    body: {
      name: input.name,
      type: input.type,
      allowedHosts: input.allowedHosts,
      ...(Object.keys(input.publicFields).length === 0 ? {} : { publicFields: input.publicFields }),
      secretFields: input.secretFields,
    },
    schema: credentialSchema,
    source: 'response of POST /projects/{projectId}/credentials',
  });
}

/** Replaces the list of hosts the secret may be sent to (`updateCredential`, JSON Merge Patch). */
export async function updateAllowedHosts(credentialId: string, etag: string, allowedHosts: readonly string[]): Promise<Credential> {
  return apiRequest({
    method: 'PATCH',
    path: `/credentials/${encodeURIComponent(credentialId)}`,
    body: { allowedHosts },
    ifMatch: etag,
    schema: credentialSchema,
    source: 'response of PATCH /credentials/{credentialId}',
  });
}

/** Stores a new version of the secret (`replaceCredentialSecret`). The public fields stay unless some are sent. */
export function replaceSecret(
  credentialId: string,
  etag: string,
  input: {
    readonly secretFields: Readonly<Record<string, string>>;
    readonly publicFields?: Readonly<Record<string, string>> | undefined;
  },
): Promise<Credential> {
  return apiRequest({
    method: 'PUT',
    path: `/credentials/${encodeURIComponent(credentialId)}/secret`,
    body: {
      secretFields: input.secretFields,
      ...(input.publicFields === undefined || Object.keys(input.publicFields).length === 0 ? {} : { publicFields: input.publicFields }),
    },
    ifMatch: etag,
    schema: credentialSchema,
    source: 'response of PUT /credentials/{credentialId}/secret',
  });
}

/** Deletes a credential (`deleteCredential`). */
export async function deleteCredential(credentialId: string, etag: string): Promise<void> {
  await apiCall({
    method: 'DELETE',
    path: `/credentials/${encodeURIComponent(credentialId)}`,
    ifMatch: etag,
    source: 'response of DELETE /credentials/{credentialId}',
  });
}

/** The brand icons of the node types of a project, by type (`listNodeTypes`). */
export async function listNodeTypeIcons(projectId: string): Promise<ServiceIcons> {
  const page = await apiRequest({
    method: 'GET',
    path: `/projects/${encodeURIComponent(projectId)}/node-types`,
    query: { limit: 200 },
    schema: nodeTypePageSchema,
    source: 'response of GET /projects/{projectId}/node-types',
  });
  const icons: Record<string, { svg: string; darkSvg?: string | undefined }> = {};
  for (const item of page.items) {
    if (item.icon !== undefined) icons[item.type] = { svg: item.icon.svg, darkSvg: item.icon.darkSvg };
  }
  return icons;
}
