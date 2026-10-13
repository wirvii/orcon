// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname } from 'node:path';

import { compile } from 'tailwindcss';
import { describe, expect, it } from 'vitest';

import { emitTailwindTheme } from '../scripts/emit.mjs';

import { model } from './helpers/model';

const require = createRequire(import.meta.url);

/** Compiles the generated theme with Tailwind and returns the CSS for the given classes. */
async function css(classes: string[]): Promise<string> {
  const theme = emitTailwindTheme(model);
  const compiler = await compile(`@import "tailwindcss/theme.css" layer(theme);\n@tailwind utilities;\n${theme}`, {
    base: dirname(require.resolve('tailwindcss/package.json')),
    loadStylesheet: async (id: string, base: string) => {
      const path = id === 'tailwindcss/theme.css' ? require.resolve('tailwindcss/theme.css') : require.resolve(id, { paths: [base] });
      return { path, base: dirname(path), content: readFileSync(path, 'utf8') };
    },
  });
  return compiler.build(classes);
}

describe('generated Tailwind theme', () => {
  it('generates utilities for semantic tokens and scale steps', async () => {
    const out = await css(['bg-surface', 'text-fg-muted', 'border-control', 'ring-focus', 'p-4', 'h-control', 'h-row', 'rounded-md', 'shadow-sm', 'text-status-attention', 'bg-status-attention-bg']);
    expect(out).toContain('var(--orcon-surface-default)');
    expect(out).toContain('var(--orcon-fg-muted)');
    expect(out).toContain('var(--orcon-border-control)');
    expect(out).toContain('var(--orcon-space-4)');
    expect(out).toContain('var(--orcon-size-control)');
    expect(out).toContain('var(--orcon-size-row)');
    expect(out).toContain('var(--orcon-radius-md)');
    expect(out).toContain('var(--orcon-elevation-sm)');
    expect(out).toContain('var(--orcon-status-attention-fg)');
    expect(out).toContain('var(--orcon-status-attention-bg)');
  });

  it('generates nothing for the default palette, off-scale spacing or default radius', async () => {
    const out = await css(['bg-red-500', 'text-slate-600', 'bg-white', 'p-7', 'rounded-3xl', 'shadow-xl']);
    expect(out).not.toMatch(/\.(bg-red-500|text-slate-600|bg-white|p-7|rounded-3xl|shadow-xl)/);
  });
});
