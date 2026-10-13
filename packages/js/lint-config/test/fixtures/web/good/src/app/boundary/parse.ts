// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ZodType } from 'zod';

/** Parses external input with its schema; a failure is a contract error. */
export function parseWith<T>(schema: ZodType<T>, input: unknown): T {
  return schema.parse(input);
}
