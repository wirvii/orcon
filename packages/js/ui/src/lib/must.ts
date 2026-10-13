// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Returns a value that the caller knows exists, or fails with a message that
 * names it. Tests and stories use it where a lookup must succeed; production
 * code handles the missing case instead.
 */
export function must<T>(value: T | null | undefined, what = 'value'): T {
  if (value === null || value === undefined) throw new Error(`Expected ${what} to exist.`);
  return value;
}
