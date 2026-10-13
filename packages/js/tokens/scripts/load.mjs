// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Loads the DTCG sources with Style Dictionary and returns a plain model that
// the emitters and the tests share. Style Dictionary parses the files and
// resolves references; this module keeps the layer, theme and density split.

import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import StyleDictionary from 'style-dictionary';

export const packageDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const src = join(packageDir, 'src');

function jsonFiles(dir) {
  return readdirSync(join(src, dir))
    .filter((f) => f.endsWith('.json'))
    .sort()
    .map((f) => join(src, dir, f));
}

/** Source files for one combination of theme and density. */
function sourcesFor(theme, density) {
  return [
    ...jsonFiles('primitives'),
    join(src, 'brand.json'),
    join(src, 'semantic', `${theme}.json`),
    join(src, 'density', `${density}.json`),
    ...jsonFiles('component'),
  ];
}

function layerOf(filePath) {
  const rel = filePath.slice(src.length + 1);
  if (rel.startsWith('primitives/')) return 'primitive';
  if (rel === 'brand.json') return 'brand';
  if (rel.startsWith('semantic/')) return 'semantic';
  if (rel.startsWith('density/')) return 'density';
  if (rel.startsWith('component/')) return 'component';
  throw new Error(`Token file outside a known layer: ${rel}`);
}

function toCssShadow(layer) {
  return `${layer.offsetX} ${layer.offsetY} ${layer.blur} ${layer.spread} ${layer.color}`;
}

/** Formats a resolved DTCG value as the CSS text it is emitted as. */
export function cssValue(type, value) {
  switch (type) {
    case 'fontFamily':
      return value.map((name) => (/^[a-z-]+$/i.test(name) ? name : `'${name}'`)).join(', ');
    case 'cubicBezier':
      return `cubic-bezier(${value.join(', ')})`;
    case 'shadow':
      return (Array.isArray(value) ? value : [value]).map(toCssShadow).join(', ');
    default:
      return String(value);
  }
}

async function resolve(theme, density) {
  const sd = new StyleDictionary({
    log: { verbosity: 'silent', warnings: 'warn', errors: { brokenReferences: 'throw' } },
    source: sourcesFor(theme, density),
    platforms: { model: { transforms: [] } },
  });
  await sd.hasInitialized;
  const { allTokens } = await sd.getPlatformTokens('model');
  return allTokens.map((token) => {
    const original = token.original.$value;
    const reference =
      typeof original === 'string' && /^\{[^}]+\}$/.test(original) ? original.slice(1, -1) : null;
    const key = token.path.join('.');
    return {
      key,
      path: token.path,
      cssName: `--orcon-${token.path.join('-')}`,
      layer: layerOf(token.filePath),
      type: token.$type,
      value: token.$value,
      css: cssValue(token.$type, token.$value),
      reference,
      role: token.$extensions?.orcon?.role ?? null,
    };
  });
}

/**
 * @typedef {object} Token
 * @property {string} key Dotted path, for example "fg.muted".
 * @property {string[]} path
 * @property {string} cssName Custom property name, for example "--orcon-fg-muted".
 * @property {'primitive'|'brand'|'semantic'|'density'|'component'} layer
 * @property {string} type DTCG type.
 * @property {unknown} value Resolved value.
 * @property {string} css Resolved value as CSS text.
 * @property {string|null} reference Dotted key this token references, if it is a reference.
 * @property {string|null} role Orcon role extension, for example "brand-exact".
 */

const themeNames = ['light', 'dark'];
const densityNames = ['comfortable', 'compact'];

/**
 * @returns {Promise<{
 *   scales: Token[], brand: Token[], primitives: Token[],
 *   themes: Record<'light'|'dark', Token[]>,
 *   densities: Record<'comfortable'|'compact', Token[]>,
 *   component: Token[],
 * }>}
 */
export async function loadModel() {
  const light = await resolve('light', 'comfortable');
  const dark = await resolve('dark', 'compact');
  const only = (tokens, layer) => tokens.filter((t) => t.layer === layer);
  const primitives = only(light, 'primitive');
  return {
    primitives,
    // Scales are the primitives that are exposed as variables; color ramps and
    // theme-specific shadow definitions stay build inputs.
    scales: primitives.filter((t) => t.type !== 'color' && t.path[0] !== 'shadow'),
    brand: only(light, 'brand'),
    themes: { light: only(light, 'semantic'), dark: only(dark, 'semantic') },
    densities: { comfortable: only(light, 'density'), compact: only(dark, 'density') },
    component: only(light, 'component'),
  };
}

export function readPairs() {
  const file = join(src, 'pairs.json');
  return JSON.parse(readFileSync(file, 'utf8')).pairs;
}

export { themeNames, densityNames };
