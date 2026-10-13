// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { model, pairs, themeNames } from './helpers/model';

/** Semantic color tokens that appear in no pair, given the declared pairs. */
export function uncovered(colorKeys: string[], declared: Array<{ fg: string; bg: string }>): string[] {
  const used = new Set(declared.flatMap((p) => [p.fg, p.bg]));
  return colorKeys.filter((key) => !used.has(key));
}

describe('contrast pair coverage', () => {
  it.each(themeNames)('puts every semantic color token of the %s theme in a pair', (theme) => {
    const keys = model.themes[theme].filter((t) => t.type === 'color').map((t) => t.key);
    expect(uncovered(keys, pairs)).toEqual([]);
  });

  it('gives every decorative or exempt pair a reason', () => {
    for (const pair of pairs.filter((p) => p.kind === 'decorative' || p.kind === 'disabled-exempt')) {
      expect(pair.reason, `${pair.fg} on ${pair.bg}`).toBeTruthy();
    }
  });

  it('reports a token that no pair covers', () => {
    expect(uncovered(['a', 'b'], [{ fg: 'a', bg: 'c' }])).toEqual(['b']);
  });

  it('declares only tokens that exist', () => {
    const known = new Set(model.themes.light.map((t) => t.key));
    for (const pair of pairs) {
      expect(known.has(pair.fg), pair.fg).toBe(true);
      expect(known.has(pair.bg), pair.bg).toBe(true);
    }
  });
});
