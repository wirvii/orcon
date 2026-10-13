// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ZodType } from 'zod';

import { tryParse } from './parse';

/** Why a clipboard read produced nothing usable. */
export type ClipboardFailure = 'denied' | 'empty' | 'not-json' | 'invalid';

export type ClipboardRead<T> = { readonly ok: true; readonly value: T } | { readonly ok: false; readonly reason: ClipboardFailure };

/** Puts text on the clipboard. Returns false when the browser refuses. */
export async function writeClipboardText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

/** Puts a JSON document on the clipboard. */
export function writeClipboardJson(value: unknown): Promise<boolean> {
  return writeClipboardText(JSON.stringify(value));
}

/**
 * Reads the clipboard as a JSON document and parses it with the schema (orcon-web
 * ST-8: paste accepts only a document it understands). The reason says what to tell
 * the person: permission refused, nothing there, not JSON, or not the expected shape.
 */
export async function readClipboardJson<T>(schema: ZodType<T>): Promise<ClipboardRead<T>> {
  let text: string;
  try {
    text = await navigator.clipboard.readText();
  } catch {
    return { ok: false, reason: 'denied' };
  }
  if (text.trim() === '') return { ok: false, reason: 'empty' };
  let json: unknown;
  try {
    json = JSON.parse(text);
  } catch {
    return { ok: false, reason: 'not-json' };
  }
  const parsed = tryParse(schema, json, 'clipboard');
  return parsed.ok ? { ok: true, value: parsed.value } : { ok: false, reason: 'invalid' };
}
