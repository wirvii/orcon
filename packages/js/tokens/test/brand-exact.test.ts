// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import type { Pair } from './helpers/contrast';
import { colorValues, model, pairs, themeNames } from './helpers/model';

// The three brand hues that are never text and never behind text. Navy and
// slate are part of the neutral ramp and may carry text.
const restricted = ['brand.green', 'brand.green-dark', 'brand.cyan'];
const restrictedHex = new Set(
  model.brand.filter((t) => restricted.includes(t.key)).map((t) => t.css.toUpperCase()),
);

/** Text pairs whose foreground or background is a restricted brand hue. */
export function brandTextViolations(
  declared: Pair[],
  values: Record<string, string>,
  forbidden: Set<string>,
): string[] {
  return declared
    .filter((p) => p.kind === 'text')
    .filter((p) => forbidden.has((values[p.fg] ?? '').toUpperCase()) || forbidden.has((values[p.bg] ?? '').toUpperCase()))
    .map((p) => `${p.fg} on ${p.bg}`);
}

describe('exact brand hues', () => {
  it('has the three restricted hues', () => {
    expect([...restrictedHex].sort()).toEqual(['#06B6D4', '#16A34A', '#22C55E']);
  });

  it.each(themeNames)('keeps them out of every text pair in the %s theme', (theme) => {
    expect(brandTextViolations(pairs, colorValues(theme), restrictedHex)).toEqual([]);
  });

  it.each(themeNames)('keeps text tokens from referencing them in the %s theme', (theme) => {
    const textKeys = new Set(pairs.filter((p) => p.kind === 'text').flatMap((p) => [p.fg, p.bg]));
    const offenders = model.themes[theme]
      .filter((t) => textKeys.has(t.key))
      .filter((t) => t.reference !== null && restricted.includes(t.reference));
    expect(offenders.map((t) => t.key)).toEqual([]);
  });

  it.each(themeNames)('limits their use to canvas lines and selection in the %s theme', (theme) => {
    const users = model.themes[theme]
      .filter((t) => restrictedHex.has(t.css.toUpperCase()))
      .map((t) => t.key)
      .sort();
    expect(users).toEqual(['canvas.edge-active', 'canvas.selection']);
  });

  it('detects a text pair that uses a restricted hue', () => {
    const values = { 'x.fg': '#22c55e', 'x.bg': '#ffffff' };
    const bad: Pair[] = [{ fg: 'x.fg', bg: 'x.bg', kind: 'text', min: 4.5 }];
    expect(brandTextViolations(bad, values, restrictedHex)).toEqual(['x.fg on x.bg']);
    expect(brandTextViolations([{ ...bad[0], kind: 'ui' } as Pair], values, restrictedHex)).toEqual([]);
  });
});
