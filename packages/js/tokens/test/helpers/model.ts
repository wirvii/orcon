// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { loadModel, readPairs } from '../../scripts/load.mjs';

import type { Pair } from './contrast';

export interface Token {
  key: string;
  path: string[];
  cssName: string;
  layer: 'primitive' | 'brand' | 'semantic' | 'density' | 'component';
  type: string;
  value: unknown;
  css: string;
  reference: string | null;
  role: string | null;
}

export interface Model {
  primitives: Token[];
  scales: Token[];
  brand: Token[];
  themes: Record<'light' | 'dark', Token[]>;
  densities: Record<'comfortable' | 'compact', Token[]>;
  component: Token[];
}

export const model = (await loadModel()) as Model;
export const pairs = readPairs() as Pair[];
export const themeNames = ['light', 'dark'] as const;

/** Map of token key to resolved CSS value for one theme (colors only). */
export function colorValues(theme: 'light' | 'dark'): Record<string, string> {
  return Object.fromEntries(
    model.themes[theme].filter((t) => t.type === 'color').map((t) => [t.key, t.css]),
  );
}
