// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import type { Credential } from '../model/credential';

export type { Credential } from '../model/credential';

/**
 * A credential as the API answers it (`Credential`, SPEC-004 R-CRED-01). Fields the screens do not read are kept
 * loose, so a newer server does not break an older page.
 */
export const credentialSchema: z.ZodType<Credential> = z.looseObject({
  id: z.string().min(1),
  projectId: z.string().min(1),
  owned: z.boolean(),
  name: z.string(),
  type: z.string(),
  status: z.enum(['OK', 'NEEDS_SECRET', 'NEEDS_REAUTH', 'DISABLED']),
  allowedHosts: z.array(z.string()),
  versionCount: z.number().int(),
  secret: z.looseObject({ set: z.boolean(), updatedAt: z.string().optional() }),
  publicFields: z.record(z.string(), z.string()).optional(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

export const credentialPageSchema = z.object({
  items: z.array(credentialSchema),
  nextPageToken: z.string().max(1024).optional(),
});

/** A credential with the entity tag it was read under: the `If-Match` of the change that follows. */
export interface CredentialWithTag {
  readonly credential: Credential;
  readonly etag: string;
}

/** What a person types to create a credential. The secret values travel once, in the request body, and are not kept. */
export interface NewCredential {
  readonly name: string;
  /** The connector reference of the service. */
  readonly type: string;
  readonly allowedHosts: readonly string[];
  readonly publicFields: Readonly<Record<string, string>>;
  readonly secretFields: Readonly<Record<string, string>>;
}

/** A node type as `listNodeTypes` answers it, reduced to what the services list reads: its type and its brand icon. */
export const nodeTypePageSchema = z.object({
  items: z.array(
    z.looseObject({
      type: z.string(),
      name: z.string(),
      icon: z.looseObject({ svg: z.string(), darkSvg: z.string().optional() }).optional(),
    }),
  ),
});
