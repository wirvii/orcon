// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { parseWith } from './parse';

/** The one place that reads browser storage and parses what it finds. */
export function readStoredNames(key: string): string[] {
  const raw = window.localStorage.getItem(key);
  if (raw === null) return [];
  return parseWith(z.array(z.string()), JSON.parse(raw));
}
