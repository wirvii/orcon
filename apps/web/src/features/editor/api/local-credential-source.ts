// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { type CredentialStore, localCredentialStore } from '../../../app/credential-store/store';

import type { CredentialSource } from './credential-source';

/** The project the preview's credential list is read for: the local store keeps one list for all of them. */
const PREVIEW_PROJECT = 'preview';

/**
 * The editor's credential widget over a store. The preview uses the shared local store, so a credential
 * created on the credentials screen is offered to a node too; tests pass their own store to start empty.
 */
export function createLocalCredentialSource(store: CredentialStore = localCredentialStore): CredentialSource {
  return {
    list: () => store.list(PREVIEW_PROJECT),
    create: (input) => store.create(PREVIEW_PROJECT, input),
  };
}
