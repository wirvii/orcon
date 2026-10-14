// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Copies every approved brand asset to the places the manifest declares in `copies`
// (brand/manifest.json). The files under brand/ stay the only source: a copy is
// never edited by hand, and `make brand-check` (BC-2) fails when a copy differs
// from its asset.
//
//   node scripts/brand-sync.mjs          write the copies
//   node scripts/brand-sync.mjs --check  fail when a copy is missing or differs

import { copyFileSync, existsSync, mkdirSync, readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../../..');
const manifest = JSON.parse(readFileSync(join(repositoryRoot, 'brand/manifest.json'), 'utf8'));
const check = process.argv.includes('--check');

let wrong = 0;
let count = 0;
for (const asset of manifest.assets) {
  if (asset.approval?.status !== 'approved') {
    if (asset.copies.length > 0) {
      console.error(`${asset.path} has copies but is not approved`);
      wrong++;
    }
    continue;
  }
  const source = join(repositoryRoot, asset.path);
  for (const copy of asset.copies) {
    const target = join(repositoryRoot, copy);
    count++;
    if (check) {
      if (!existsSync(target) || !readFileSync(target).equals(readFileSync(source))) {
        console.error(`${copy} is not a copy of ${asset.path}; run pnpm --filter @orcon/ui brand:sync`);
        wrong++;
      }
      continue;
    }
    mkdirSync(dirname(target), { recursive: true });
    copyFileSync(source, target);
  }
}

if (wrong > 0) process.exit(1);
console.log(`${check ? 'checked' : 'wrote'} ${count} brand copies`);
