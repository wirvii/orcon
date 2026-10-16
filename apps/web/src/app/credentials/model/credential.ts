// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A credential as the screens read it (`Credential` of the API, SPEC-004 R-CRED-01): what a person may know
 * about it and the state of its secret, never a secret value.
 */
export interface Credential {
  readonly id: string;
  readonly projectId: string;
  /** Whether the project the list belongs to owns it; a credential shared with the project can be used but not changed. */
  readonly owned: boolean;
  readonly name: string;
  /** The connector reference the credential serves, for example `orcon.http@1`. */
  readonly type: string;
  readonly status: 'OK' | 'NEEDS_SECRET' | 'NEEDS_REAUTH' | 'DISABLED';
  /** The hosts the secret may be sent to; a request to any other host never receives it. */
  readonly allowedHosts: readonly string[];
  readonly versionCount: number;
  readonly secret: {
    readonly set: boolean;
    readonly updatedAt?: string | undefined;
  };
  /** Present when one credential is read; absent from list items. */
  readonly publicFields?: Readonly<Record<string, string>> | undefined;
  readonly createdAt: string;
  readonly updatedAt: string;
}
