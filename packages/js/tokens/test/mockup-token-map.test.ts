// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

import { colorValues, model } from './helpers/model';

const here = dirname(fileURLToPath(import.meta.url));
const mapPath = join(here, '..', '..', '..', '..', 'docs', 'design-system', 'mockups', 'ux-2026-10', 'token-map.md');

/** The cells of every table row under a heading, without the header and the divider. */
function rows(markdown: string, heading: string): string[][] {
  const start = markdown.indexOf(`## ${heading}`);
  if (start < 0) throw new Error(`token-map.md has no "${heading}" section`);
  const next = markdown.indexOf('\n## ', start + 1);
  const section = markdown.slice(start, next < 0 ? undefined : next);
  return section
    .split('\n')
    .filter((line) => line.startsWith('|'))
    .map((line) => line.split('|').slice(1, -1).map((cell) => cell.trim()))
    .filter((cells) => cells.length > 0 && !/^-+$/.test(cells[0] ?? '') && cells[0] !== 'mockup');
}

const markdown = readFileSync(mapPath, 'utf8');
const colorRows = rows(markdown, 'Colors');
const scaleRows = rows(markdown, 'Radius and type scale');

const dark = colorValues('dark');
const light = colorValues('light');
const brand = Object.fromEntries(model.brand.map((token) => [token.key, token.css.toUpperCase()]));
const scale = Object.fromEntries(model.scales.map((token) => [token.key, token.css]));

/** The value a token has in a theme: brand colors do not change with the theme. */
const valueOf = (theme: Record<string, string>, token: string): string | undefined => (theme[token] ?? brand[token])?.toUpperCase();

describe('mockup token map', () => {
  it('every row names an existing token', () => {
    expect(colorRows.length).toBeGreaterThan(30);
    const missing = colorRows.filter((cells) => valueOf(dark, cells[3] ?? '') === undefined).map((cells) => cells[3]);
    expect(missing).toEqual([]);
    const missingScale = scaleRows.filter((cells) => scale[cells[2] ?? ''] === undefined).map((cells) => cells[2]);
    expect(missingScale).toEqual([]);
  });

  it('dark values match', () => {
    const wrong = colorRows.filter((cells) => valueOf(dark, cells[3] ?? '') !== (cells[1] ?? '').toUpperCase()).map((cells) => `${cells[0]}: ${cells[1]} is not ${cells[3]}`);
    expect(wrong).toEqual([]);
  });

  it('light values match', () => {
    const wrong = colorRows.filter((cells) => valueOf(light, cells[3] ?? '') !== (cells[2] ?? '').toUpperCase()).map((cells) => `${cells[0]}: ${cells[2]} is not ${cells[3]}`);
    expect(wrong).toEqual([]);
  });

  it('radius and type sizes match their tokens', () => {
    const wrong = scaleRows.filter((cells) => scale[cells[2] ?? ''] !== cells[1]).map((cells) => `${cells[0]}: ${cells[1]} is not ${cells[2]}`);
    expect(wrong).toEqual([]);
  });

  it('status families include attention for uncertain', () => {
    const attention = colorRows.filter((cells) => (cells[3] ?? '').startsWith('status.attention.'));
    expect(attention.map((cells) => cells[3])).toEqual(['status.attention.fg', 'status.attention.bg']);
    expect(dark['status.attention.fg']).toBeDefined();
    expect(light['status.attention.bg']).toBeDefined();
  });
});
