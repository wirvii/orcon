// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ZodType } from 'zod';

import { ContractError } from '../errors/contract-error';

/**
 * Parses external input with its schema (orcon-web TS-3). Responses, event payloads,
 * storage, clipboard and the URL are `unknown` until they pass through here. A value
 * that does not match is a contract error.
 *
 * @param source Where the value came from, for the error: "response of GET /flows".
 */
export function parseWith<T>(schema: ZodType<T>, input: unknown, source = 'value'): T {
  const result = schema.safeParse(input);
  if (!result.success) throw new ContractError(source, result.error.issues);
  return result.data;
}

/** The outcome of {@link tryParse}: the parsed value or the contract error. */
export type ParseResult<T> = { readonly ok: true; readonly value: T } | { readonly ok: false; readonly error: ContractError };

/** Like {@link parseWith}, for callers that handle the failure instead of throwing. */
export function tryParse<T>(schema: ZodType<T>, input: unknown, source = 'value'): ParseResult<T> {
  const result = schema.safeParse(input);
  return result.success ? { ok: true, value: result.data } : { ok: false, error: new ContractError(source, result.error.issues) };
}

/** The value a text holds as JSON, or a failure when it is not JSON. The value is `unknown`: check it before use. */
export function parseJsonText(text: string): { readonly ok: true; readonly value: unknown } | { readonly ok: false } {
  try {
    return { ok: true, value: JSON.parse(text) as unknown };
  } catch {
    return { ok: false };
  }
}

/** Whether a text is valid JSON. */
export function isJsonText(text: string): boolean {
  return parseJsonText(text).ok;
}
