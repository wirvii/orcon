// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { CredentialNameTaken, type CredentialSource, type CredentialSummary } from './credential-source';

const SEED: readonly CredentialSummary[] = [
  { id: 'cred-slack', name: 'Slack incoming webhook', type: 'httpHeaderAuth', updatedAt: '2026-10-09T14:20:00Z' },
  { id: 'cred-inventory', name: 'Inventory API key', type: 'httpHeaderAuth', updatedAt: '2026-09-28T08:05:00Z' },
];

/**
 * A credential source that keeps the list in memory and makes no request. The preview of the editor
 * uses it: creating a credential adds a name to the list, but the secret values are dropped on the
 * spot (nothing here could keep them safely), and a credential cannot be tested.
 */
export function createLocalCredentialSource(seed: readonly CredentialSummary[] = SEED): CredentialSource {
  const held = [...seed];
  let counter = 0;
  return {
    list: () => Promise.resolve([...held]),
    create: ({ name, type }) => {
      if (held.some((credential) => credential.name === name)) return Promise.reject(new CredentialNameTaken(name));
      counter += 1;
      const created: CredentialSummary = { id: `cred-local-${String(counter)}`, name, type, updatedAt: new Date().toISOString() };
      held.push(created);
      return Promise.resolve(created);
    },
  };
}
