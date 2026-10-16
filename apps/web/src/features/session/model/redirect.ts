// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The address a person is sent to after signing in (orcon-web RT-10). Only a path of this same
 * origin is accepted: an absolute URL, a protocol-relative `//host` path, a backslash (some
 * browsers read `/\host` as `//host`), a `javascript:` URL and any control character are refused.
 * A refused value is no redirect at all, so the person lands on the default page.
 */
export function safeRedirect(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined;
  if (!value.startsWith('/')) return undefined;
  if (value.startsWith('//') || value.includes('\\')) return undefined;
  for (const character of value) {
    const code = character.codePointAt(0) ?? 0;
    if (code < 0x20 || code === 0x7f) return undefined;
  }
  return value;
}
