// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/** Reports a contract error without the response content. */
export function reportContractError(requestId: string): void {
  console.error(`Contract error for request ${requestId}`);
}
