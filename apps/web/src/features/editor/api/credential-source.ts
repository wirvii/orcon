// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { CredentialSummary, NewCredential } from '../../../app/credential-store/store';

// The credential shapes live with the credential store, which the credentials screens share with the
// editor. They are re-exported so the editor's forms keep one import path.
export { CredentialNameTaken, type CredentialSummary, type NewCredential } from '../../../app/credential-store/store';

/**
 * Where the editor reads the project's credentials and creates new ones. This is the whole surface
 * the credential widget needs (SPEC-014 R-ED-CRED-01..02): the list of credentials the person can
 * use (`credential:use`) and creating one. The implementation is the app's credential store (the same
 * one the credentials screens use); `test` is absent while the store cannot test a credential, and the
 * widget then says so instead of offering a button that cannot work.
 */
export interface CredentialSource {
  list: () => Promise<readonly CredentialSummary[]>;
  /** Rejects with `CredentialNameTaken` when the name exists. */
  create: (input: NewCredential) => Promise<CredentialSummary>;
  test?: (credentialId: string) => Promise<{ readonly ok: boolean; readonly message?: string }>;
}
