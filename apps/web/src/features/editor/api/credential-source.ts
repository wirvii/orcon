// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/** A credential as the editor lists it: never the secret values (ADR-0043). */
export interface CredentialSummary {
  readonly id: string;
  readonly name: string;
  /** A credential type id, for example `httpHeaderAuth`. */
  readonly type: string;
  /** ISO 8601 instant of the last change. */
  readonly updatedAt: string;
}

export interface NewCredential {
  readonly name: string;
  readonly type: string;
  /** The values of the type's fields, secret ones included. They travel once, to the source, and are not kept. */
  readonly fields: Readonly<Record<string, string>>;
}

/** The server refused the name because the project already has a credential with it. */
export class CredentialNameTaken extends Error {
  readonly status = 409;
  readonly code = 'conflict';
  constructor(name: string) {
    super(`A credential named ${name} already exists`);
  }
}

/**
 * Where the editor reads the project's credentials and creates new ones. This is the whole surface
 * the credential widget needs (SPEC-014 R-ED-CRED-01..02): the list of credentials the person can
 * use (`credential:use`) and creating one. Connecting it to the server later means writing one more
 * implementation (the SDK operations `listCredentials` and `createCredential` of SPEC-005) and
 * choosing it in `queries.ts`; the widget does not change. `test` is absent while the source cannot
 * test a credential, and the widget then says so instead of offering a button that cannot work.
 */
export interface CredentialSource {
  list: () => Promise<readonly CredentialSummary[]>;
  /** Rejects with `CredentialNameTaken` when the name exists. */
  create: (input: NewCredential) => Promise<CredentialSummary>;
  test?: (credentialId: string) => Promise<{ readonly ok: boolean; readonly message?: string }>;
}
