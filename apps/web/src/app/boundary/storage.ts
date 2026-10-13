// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ZodType } from 'zod';

import { tryParse } from './parse';

/**
 * Reads a value saved by {@link writeStored}. Anything unreadable (storage blocked,
 * nothing saved, broken JSON, a value that no longer matches the schema) gives the
 * fallback: a saved preference must never break the page.
 */
export function readStored<T>(key: string, schema: ZodType<T>, fallback: T): T {
  let raw: string | null;
  try {
    raw = window.localStorage.getItem(key);
  } catch {
    return fallback;
  }
  if (raw === null) return fallback;
  let json: unknown;
  try {
    json = JSON.parse(raw);
  } catch {
    return fallback;
  }
  const parsed = tryParse(schema, json, `saved value ${key}`);
  return parsed.ok ? parsed.value : fallback;
}

/** Saves a JSON-serializable value. Returns false when the browser refuses (private mode, quota). */
export function writeStored(key: string, value: unknown): boolean {
  try {
    window.localStorage.setItem(key, JSON.stringify(value));
    return true;
  } catch {
    return false;
  }
}

/** Removes a saved value. Never throws. */
export function removeStored(key: string): void {
  try {
    window.localStorage.removeItem(key);
  } catch {
    // Nothing to remove when storage is blocked.
  }
}
