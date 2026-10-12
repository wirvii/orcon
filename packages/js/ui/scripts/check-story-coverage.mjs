// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Story coverage gate.
//
// Every component has a `<Name>.meta.ts` that exports `states`, a
// `<Name>.stories.tsx` with one story per declared state, and a `Matrix` story.
// The stories are read from the built Storybook index, so this checks what
// Storybook actually serves, not what the source files claim.
//
//   node scripts/check-story-coverage.mjs [--strict]
//
// Run `pnpm --filter @orcon/ui build-storybook` first. Without --strict, base
// components of the inventory that do not exist yet are listed but do not fail
// the run; --strict makes them fail.

import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const strict = process.argv.includes('--strict');
const indexPath = join(root, 'storybook-static/index.json');

if (!existsSync(indexPath)) {
  console.error('story coverage: storybook-static/index.json not found. Run "pnpm --filter @orcon/ui build-storybook" first.');
  process.exit(1);
}

const index = JSON.parse(readFileSync(indexPath, 'utf8'));
const storiesByFile = new Map();
for (const entry of Object.values(index.entries)) {
  if (entry.type !== 'story') continue;
  const suffix = entry.id.slice(entry.id.indexOf('--') + 2);
  if (!storiesByFile.has(entry.importPath)) storiesByFile.set(entry.importPath, new Set());
  storiesByFile.get(entry.importPath).add(suffix);
}

function walk(dir) {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    return statSync(path).isDirectory() ? walk(path) : [path];
  });
}

const componentsRoot = join(root, 'src/components');
const files = walk(componentsRoot);
const metaFiles = files.filter((file) => file.endsWith('.meta.ts'));
const storyFiles = files.filter((file) => file.endsWith('.stories.tsx'));
const problems = [];

const rel = (file) => relative(root, file);

for (const storyFile of storyFiles) {
  const meta = storyFile.replace(/\.stories\.tsx$/, '.meta.ts');
  if (!existsSync(meta)) problems.push(`${rel(storyFile)} has no ${rel(meta)}`);
}

let storyCount = 0;
for (const metaFile of metaFiles) {
  const source = readFileSync(metaFile, 'utf8');
  const match = source.match(/export const states = \[([^\]]*)\] as const;/);
  if (!match) {
    problems.push(`${rel(metaFile)} must export "states" as a literal array: export const states = [...] as const;`);
    continue;
  }
  const states = [...match[1].matchAll(/'([a-z0-9-]+)'/g)].map((m) => m[1]);
  if (states.length === 0) {
    problems.push(`${rel(metaFile)} declares no states`);
    continue;
  }
  if (!/export const pairs = |export \{[^}]*\bpairs\b[^}]*\} from /.test(source)) problems.push(`${rel(metaFile)} must export "pairs"`);

  const storyFile = metaFile.replace(/\.meta\.ts$/, '.stories.tsx');
  const importPath = `./${rel(storyFile)}`;
  const present = storiesByFile.get(importPath);
  if (!present) {
    problems.push(`${rel(metaFile)}: no stories found in the Storybook index for ${importPath}`);
    continue;
  }
  for (const state of [...states, 'matrix']) {
    storyCount += 1;
    if (!present.has(state)) problems.push(`${importPath}: missing story "${state}"`);
  }
}

// Inventory: base components that must exist before the design system can be reviewed.
const inventory = JSON.parse(readFileSync(join(root, 'scripts/inventory.json'), 'utf8'));
const have = new Set(metaFiles.map((file) => file.split('/').pop().replace(/\.meta\.ts$/, '')));
const remaining = inventory.filter((name) => !have.has(name));

if (remaining.length > 0) {
  const text = `inventory components without meta and stories (${remaining.length}/${inventory.length}): ${remaining.join(', ')}`;
  if (strict) problems.push(text);
  else console.warn(`story coverage: pending ${text}`);
}

if (problems.length > 0) {
  console.error(`story coverage failed:\n${problems.map((problem) => `  - ${problem}`).join('\n')}`);
  process.exit(1);
}
console.log(`story coverage ok: ${metaFiles.length} components, ${storyCount} required stories present`);
