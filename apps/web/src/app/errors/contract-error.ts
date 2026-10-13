// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { z } from 'zod';

/**
 * External input that does not match its schema (orcon-web ER-5). The message names
 * where the value came from; the issues carry paths and rules but never the value,
 * so nothing sensitive can end up in a log or on screen.
 */
export class ContractError extends Error {
  override readonly name = 'ContractError';
  /** Where the value came from, for example "response of GET /flows". */
  readonly source: string;
  readonly issues: readonly z.core.$ZodIssue[];

  constructor(source: string, issues: readonly z.core.$ZodIssue[]) {
    super(`${source} does not match its schema`);
    this.source = source;
    this.issues = issues;
  }
}
