// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Message catalogs for @orcon/ui.
//
//   node scripts/i18n.mjs write   extract messages from src/, write en.json and the
//                                 pseudo-locale xx-LS.json (es.json is edited by hand)
//   node scripts/i18n.mjs check   fail when a catalog is out of date, when en and es
//                                 differ in keys or placeholders, or when a message is
//                                 not valid ICU MessageFormat
//
// Source of truth: the defineMessages/formatMessage calls in src/**. en.json is
// generated from their default messages, so a key without a use cannot survive.

import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { parse, TYPE } from '@formatjs/icu-messageformat-parser';
import { printAST } from '@formatjs/icu-messageformat-parser/printer.js';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const i18nDir = join(root, 'src/i18n');
const formatjs = join(root, 'node_modules/.bin/formatjs');

const sorted = (object) => Object.fromEntries(Object.entries(object).sort(([a], [b]) => a.localeCompare(b)));
const serialize = (object) => `${JSON.stringify(sorted(object), null, 2)}\n`;
const readCatalog = (name) => JSON.parse(readFileSync(join(i18nDir, name), 'utf8'));

const scratch = mkdtempSync(join(tmpdir(), 'orcon-i18n-'));
process.on('exit', () => rmSync(scratch, { recursive: true, force: true }));

/** Extracts messages from the sources: { id: { defaultMessage, description } }. */
function extract() {
  const out = join(scratch, 'extracted.json');
  execFileSync(
    formatjs,
    [
      'extract',
      'src/**/*.{ts,tsx}',
      '--ignore',
      'src/**/*.stories.tsx',
      '--ignore',
      'src/**/*.d.ts',
      '--out-file',
      out,
    ],
    { cwd: root, stdio: ['ignore', 'inherit', 'inherit'] },
  );
  return JSON.parse(readFileSync(out, 'utf8'));
}

/** Builds the pseudo-locale (long strings) catalog from en. */
function pseudoFrom(en) {
  const input = join(scratch, 'en.json');
  const out = join(scratch, 'pseudo.json');
  writeFileSync(input, JSON.stringify(en));
  // The CLI emits the pseudo-localized messages as parsed ICU; print them back to strings
  // so the catalog has the same shape as en.json and es.json.
  execFileSync(formatjs, ['compile', input, '--format', 'simple', '--ast', '--pseudo-locale', 'xx-LS', '--out-file', out], {
    cwd: root,
    stdio: ['ignore', 'inherit', 'inherit'],
  });
  const ast = JSON.parse(readFileSync(out, 'utf8'));
  return Object.fromEntries(Object.entries(ast).map(([id, elements]) => [id, printAST(elements)]));
}

function placeholders(ast, found = new Set()) {
  for (const element of ast) {
    if (element.type === TYPE.literal || element.type === TYPE.pound) continue;
    if (element.value) found.add(`${element.value}:${element.type}`);
    if (element.type === TYPE.plural || element.type === TYPE.select) {
      for (const option of Object.values(element.options)) placeholders(option.value, found);
    }
    if (element.type === TYPE.tag) placeholders(element.children, found);
  }
  return found;
}

function enFromSources() {
  const extracted = extract();
  const missingDescription = Object.entries(extracted)
    .filter(([, message]) => !message.description)
    .map(([id]) => id);
  if (missingDescription.length > 0) {
    throw new Error(`Messages without a translator description: ${missingDescription.join(', ')}`);
  }
  return sorted(Object.fromEntries(Object.entries(extracted).map(([id, message]) => [id, message.defaultMessage])));
}

const command = process.argv[2];

if (command === 'write') {
  const en = enFromSources();
  writeFileSync(join(i18nDir, 'en.json'), serialize(en));
  writeFileSync(join(i18nDir, 'xx-LS.json'), serialize(pseudoFrom(en)));
  console.log(`i18n: wrote en.json and xx-LS.json (${Object.keys(en).length} messages)`);
} else if (command === 'check') {
  const problems = [];
  const en = enFromSources();
  const catalogEn = readCatalog('en.json');
  const catalogEs = readCatalog('es.json');

  for (const id of Object.keys(en)) {
    if (!(id in catalogEn)) problems.push(`en.json is missing "${id}" (used in the code); run i18n:write`);
    else if (catalogEn[id] !== en[id]) problems.push(`en.json differs from the default message of "${id}"; run i18n:write`);
  }
  for (const id of Object.keys(catalogEn)) {
    if (!(id in en)) problems.push(`en.json has "${id}" but no code uses it; run i18n:write`);
  }
  for (const id of Object.keys(catalogEn)) {
    if (!(id in catalogEs)) problems.push(`es.json is missing "${id}"`);
  }
  for (const id of Object.keys(catalogEs)) {
    if (!(id in catalogEn)) problems.push(`es.json has "${id}" which en.json does not have`);
  }
  for (const [name, catalog] of [
    ['en', catalogEn],
    ['es', catalogEs],
  ]) {
    for (const [id, message] of Object.entries(catalog)) {
      try {
        parse(message);
      } catch (error) {
        problems.push(`${name}.json "${id}" is not valid ICU MessageFormat: ${error.message}`);
      }
    }
  }
  for (const id of Object.keys(catalogEn)) {
    if (!(id in catalogEs)) continue;
    try {
      const a = [...placeholders(parse(catalogEn[id]))].sort().join(',');
      const b = [...placeholders(parse(catalogEs[id]))].sort().join(',');
      if (a !== b) problems.push(`"${id}" uses different placeholders in en (${a || 'none'}) and es (${b || 'none'})`);
    } catch {
      // Already reported as invalid ICU above.
    }
  }
  const pseudo = serialize(pseudoFrom(catalogEn));
  if (readFileSync(join(i18nDir, 'xx-LS.json'), 'utf8') !== pseudo) {
    problems.push('xx-LS.json is out of date; run i18n:write');
  }

  if (problems.length > 0) {
    console.error(`i18n:check failed:\n${problems.map((problem) => `  - ${problem}`).join('\n')}`);
    process.exit(1);
  }
  console.log(`i18n:check ok (${Object.keys(catalogEn).length} messages in en and es)`);
} else {
  console.error('usage: node scripts/i18n.mjs <write|check>');
  process.exit(2);
}
