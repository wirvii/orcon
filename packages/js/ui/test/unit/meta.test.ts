// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import pairsSource from '../../../tokens/src/pairs.json';

interface Meta {
  states: readonly string[];
  pairs: readonly { fg: string; bg: string }[];
}

const metas = import.meta.glob<Meta>('../../src/components/**/*.meta.ts', { eager: true });

// The certified pairs live in the tokens package; a component may only use a declared pair.
const declared = new Set(pairsSource.pairs.map(({ fg, bg }) => `${fg} on ${bg}`));

describe('component meta', () => {
  it('finds component metas', () => {
    expect(Object.keys(metas).length).toBeGreaterThan(0);
  });

  describe.each(Object.entries(metas))('%s', (path, meta) => {
    it('declares unique kebab-case states that include "default"', () => {
      expect(meta.states.length).toBeGreaterThan(0);
      expect(new Set(meta.states).size).toBe(meta.states.length);
      for (const state of meta.states) expect(state).toMatch(/^[a-z][a-z0-9]*(-[a-z0-9]+)*$/);
      expect(meta.states).toContain('default');
    });

    it('uses only fg/bg pairs that @orcon/tokens certifies', () => {
      const undeclared = meta.pairs.map(({ fg, bg }) => `${fg} on ${bg}`).filter((key) => !declared.has(key));
      expect(undeclared, `${path} uses undeclared pairs`).toEqual([]);
    });
  });
});
