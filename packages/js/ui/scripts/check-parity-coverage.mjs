// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Parity coverage gate.
//
// Reads the parity table of the canvas reference (`canvas.md#parity`), takes the rows that this
// repository owns (owner starts with SPEC-007) and checks, for each one, that
//   - the built Storybook has the story `editor-parity--p-NN`, and
//   - the visual baselines of its end state exist in light and in dark.
// It fails naming the row that is missing something.
//
//   node scripts/check-parity-coverage.mjs
//
// Run `pnpm --filter @orcon/ui build-storybook` first. Baselines are read from
// visual/__screenshots__ (they are made by the visual-baselines workflow).

import { existsSync, readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const repo = resolve(root, '../../..');
const tablePath = join(repo, '.agents/skills/orcon-ui/references/canvas.md');
const indexPath = join(root, 'storybook-static/index.json');
const shots = join(root, 'visual/__screenshots__');

for (const path of [tablePath, indexPath]) {
  if (!existsSync(path)) {
    console.error(`parity coverage: ${path} not found.${path === indexPath ? ' Run "pnpm --filter @orcon/ui build-storybook" first.' : ''}`);
    process.exit(1);
  }
}

const markdown = readFileSync(tablePath, 'utf8');
const section = markdown.slice(markdown.indexOf('## Parity'));
const rows = [...section.matchAll(/^\| (P-\d{2}) \| ([^|]*)\| ([^|]*)\| ([^|]*)\| ([^|]*)\|$/gm)].map((m) => ({
  id: m[1],
  owner: m[5].trim(),
}));
if (rows.length === 0) {
  console.error('parity coverage: no rows found in canvas.md#parity');
  process.exit(1);
}
const owned = rows.filter((row) => row.owner.startsWith('SPEC-007'));

const stories = new Set(Object.values(JSON.parse(readFileSync(indexPath, 'utf8')).entries).filter((entry) => entry.type === 'story').map((entry) => entry.id));
const problems = [];
for (const row of owned) {
  const id = `editor-parity--${row.id.toLowerCase()}`;
  if (!stories.has(id)) problems.push(`${row.id}: the story "${id}" is not in storybook-static/index.json`);
  for (const theme of ['light', 'dark']) {
    const file = join(shots, `${id}-${theme}.png`);
    if (!existsSync(file)) problems.push(`${row.id}: missing the ${theme} baseline ${file.slice(root.length + 1)}`);
  }
}

if (problems.length > 0) {
  console.error(`parity coverage failed (${owned.length} rows owned here):\n${problems.map((problem) => `  - ${problem}`).join('\n')}`);
  process.exit(1);
}
console.log(`parity coverage ok: ${owned.length} rows owned here (${rows.length} in the table), each with a story and light and dark baselines`);
