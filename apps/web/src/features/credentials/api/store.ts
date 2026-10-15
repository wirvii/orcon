// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { type CredentialStore, localCredentialStore } from '../../../app/credential-store/store';

import { createServerCredentialStore } from './server-credential-store';

/**
 * Whether the credentials screens use the in-memory store. The preview does: the credential operations
 * are not in the API contract yet, so nothing is stored on the server. When they are, return `false`
 * here; nothing else changes (the screens only see the `CredentialStore` port).
 */
export function usesLocalCredentials(): boolean {
  return true;
}

/** The store the credentials screens read and change. */
export const credentialStore: CredentialStore = usesLocalCredentials() ? localCredentialStore : createServerCredentialStore();
