// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/** A credential as the screens list it: its name and type and when it last changed, never a secret value (SPEC-004 R-CRED-01). */
export interface CredentialSummary {
  readonly id: string;
  readonly name: string;
  /** A credential type id, for example `httpHeaderAuth` or `whatsappCloudApi`. */
  readonly type: string;
  /** ISO 8601 instant of the last change; replacing a secret counts as a change (R-CRED-02). */
  readonly updatedAt: string;
}

export interface NewCredential {
  readonly name: string;
  readonly type: string;
  /** The values of the type's fields, secret ones included. They travel once, to the store, and are not kept. */
  readonly fields: Readonly<Record<string, string>>;
}

/** The project already has a credential with this name (the server answers 409 conflict). */
export class CredentialNameTaken extends Error {
  readonly status = 409;
  readonly code = 'conflict';
  constructor(name: string) {
    super(`A credential named ${name} already exists`);
  }
}

/** The credential is gone: it was deleted by someone else, or the address is old. */
export class CredentialNotFound extends Error {
  readonly status = 404;
  readonly code = 'not-found';
  constructor(id: string) {
    super(`No credential ${id}`);
  }
}

/**
 * Where the app reads and changes the credentials of a project (SPEC-005 `projects/{projectId}/credentials`,
 * `credentials/{credentialId}`). Secret values go in and never come out: no method returns them.
 */
export interface CredentialStore {
  list: (projectId: string) => Promise<readonly CredentialSummary[]>;
  create: (projectId: string, input: NewCredential) => Promise<CredentialSummary>;
  /** Replaces the secret values, which makes a new version of the credential (R-CRED-02). The name and type stay. */
  rotate: (credentialId: string, fields: Readonly<Record<string, string>>) => Promise<CredentialSummary>;
  remove: (credentialId: string) => Promise<void>;
}

const SEED: readonly CredentialSummary[] = [
  { id: 'cred-slack', name: 'Slack incoming webhook', type: 'httpHeaderAuth', updatedAt: '2026-10-09T14:20:00Z' },
  { id: 'cred-inventory', name: 'Inventory API key', type: 'httpBearerAuth', updatedAt: '2026-09-28T08:05:00Z' },
  { id: 'cred-whatsapp', name: 'Support line', type: 'whatsappCloudApi', updatedAt: '2026-10-02T16:40:00Z' },
  { id: 'cred-model', name: 'Model key production', type: 'modelProviderApiKey', updatedAt: '2026-10-11T09:12:00Z' },
];

/**
 * A credential store that keeps the list in memory and makes no request. The preview uses it until
 * the credential operations are in the contract: a secret value is dropped on the spot, because nothing
 * here could keep it safely, and a credential cannot be tested. Every project shares this one list.
 */
export function createLocalCredentialStore(seed: readonly CredentialSummary[] = SEED): CredentialStore {
  const held: CredentialSummary[] = [...seed];
  let counter = 0;
  const indexOf = (id: string) => held.findIndex((credential) => credential.id === id);
  return {
    list: () => Promise.resolve([...held]),
    create: (_projectId, { name, type }) => {
      if (held.some((credential) => credential.name === name)) return Promise.reject(new CredentialNameTaken(name));
      counter += 1;
      const created: CredentialSummary = { id: `cred-local-${String(counter)}`, name, type, updatedAt: new Date().toISOString() };
      held.push(created);
      return Promise.resolve(created);
    },
    rotate: (credentialId) => {
      const index = indexOf(credentialId);
      const current = held[index];
      if (current === undefined) return Promise.reject(new CredentialNotFound(credentialId));
      const replaced: CredentialSummary = { ...current, updatedAt: new Date().toISOString() };
      held[index] = replaced;
      return Promise.resolve(replaced);
    },
    remove: (credentialId) => {
      const index = indexOf(credentialId);
      if (index < 0) return Promise.reject(new CredentialNotFound(credentialId));
      held.splice(index, 1);
      return Promise.resolve();
    },
  };
}

/** The store the preview shares between the credentials screens and the flow editor. */
export const localCredentialStore: CredentialStore = createLocalCredentialStore();
