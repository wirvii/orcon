// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { model, type Token } from './helpers/model';

function componentToken(key: string, reference: string | null): Token {
  return { key, path: key.split('.'), cssName: `--${key.replaceAll('.', '-')}`, layer: 'component', type: 'color', value: '', css: '', reference, role: null };
}

const lower = new Set(['primitive', 'brand']);

/** Component tokens that hold a raw value or reference a layer other than semantic or density. */
export function componentViolations(component: Token[], allowed: Map<string, Token['layer']>): string[] {
  return component.flatMap((t) => {
    if (t.reference === null) return [`${t.key}: raw value`];
    const layer = allowed.get(t.reference);
    if (layer !== 'semantic' && layer !== 'density') return [`${t.key}: references ${t.reference} (${layer ?? 'unknown'})`];
    return [];
  });
}

const upper = new Map<string, Token['layer']>([
  ...model.themes.light.map((t) => [t.key, t.layer] as const),
  ...model.densities.comfortable.map((t) => [t.key, t.layer] as const),
  ...model.primitives.map((t) => [t.key, t.layer] as const),
  ...model.brand.map((t) => [t.key, t.layer] as const),
]);

describe('token layers', () => {
  it('lets component tokens reference only semantic and density tokens', () => {
    expect(model.component.length).toBeGreaterThan(0);
    expect(componentViolations(model.component, upper)).toEqual([]);
  });

  it('flags a component token with a raw value or a primitive reference', () => {
    const raw = componentToken('component.x', null);
    const prim = componentToken('component.y', 'color.green.500');
    expect(componentViolations([raw, prim], upper)).toHaveLength(2);
  });

  it('builds semantic color and elevation tokens from references to primitives or brand', () => {
    for (const theme of ['light', 'dark'] as const) {
      for (const t of model.themes[theme]) {
        expect(t.reference, `${theme} ${t.key} must be a reference`).not.toBeNull();
        expect(lower.has(upper.get(t.reference as string) ?? ''), `${theme} ${t.key} -> ${t.reference}`).toBe(true);
      }
    }
  });

  it('defines every semantic token in both themes with the same type', () => {
    const shape = (tokens: Token[]) => tokens.map((t) => `${t.key}:${t.type}`).sort();
    expect(shape(model.themes.dark)).toEqual(shape(model.themes.light));
  });

  it('defines every density token in both densities with the same type', () => {
    const shape = (tokens: Token[]) => tokens.map((t) => `${t.key}:${t.type}`).sort();
    expect(shape(model.densities.compact)).toEqual(shape(model.densities.comfortable));
  });

  it('exposes brand colors with the brand-exact role', () => {
    expect(model.brand.map((t) => t.key).sort()).toEqual([
      'brand.cyan',
      'brand.green',
      'brand.green-dark',
      'brand.green-light',
      'brand.navy',
      'brand.slate',
    ]);
    expect(model.brand.every((t) => t.role === 'brand-exact')).toBe(true);
  });
});
