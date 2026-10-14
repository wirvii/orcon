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

/** Why a pasted text produced nothing: nothing there, bigger than the limit, or not JSON. */
export type PastedTextFailure = 'empty' | 'too-large' | 'not-json';

export type PastedText = { readonly ok: true; readonly value: unknown } | { readonly ok: false; readonly reason: PastedTextFailure };

/**
 * The text of a copy, cut or paste event. Reading it from the event needs no permission, unlike
 * reading the clipboard directly, and works for a key press the person made.
 */
export function clipboardEventText(event: { clipboardData: Pick<DataTransfer, 'getData'> | null }): string {
  return event.clipboardData?.getData('text/plain') ?? '';
}

/** Puts text on the clipboard through the event of a copy or a cut; returns false when the event has no clipboard. */
export function setClipboardEventText(event: { clipboardData: Pick<DataTransfer, 'setData'> | null }, text: string): boolean {
  if (!event.clipboardData) return false;
  event.clipboardData.setData('text/plain', text);
  return true;
}

/**
 * Reads pasted text as JSON without trusting its shape: callers detect and validate the format
 * with their own schema. Text over `maxBytes` is refused before it is parsed (SPEC-014 R-ED-SEC-02).
 */
export function parsePastedJson(text: string, maxBytes: number): PastedText {
  if (text.trim() === '') return { ok: false, reason: 'empty' };
  if (new TextEncoder().encode(text).length > maxBytes) return { ok: false, reason: 'too-large' };
  try {
    return { ok: true, value: JSON.parse(text) as unknown };
  } catch {
    return { ok: false, reason: 'not-json' };
  }
}
