// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ContractError } from './contract-error';

/**
 * Logs a contract error for developers: where the value came from and which paths
 * failed which rule. The value itself is never logged (orcon-web ER-5, ER-7).
 * This is the one place in the app that writes to the console.
 */
export function reportContractError(error: ContractError, requestId?: string): void {
  const failures = error.issues.map((issue) => `${issue.path.map(String).join('.') || '(root)'}: ${issue.code}`);
  console.error(`Contract error: ${error.source}${requestId === undefined ? '' : ` (request ${requestId})`}`, failures);
}
