// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Fails when the build holds a file that the Go binary would not embed. The binary embeds the build
// with `//go:embed dist`, which silently leaves out every file whose name starts with "_" or "."
// (the `.vite` folder is on purpose; a script or a style sheet is a bug: the page asks for it and
// gets a 404). The route plugin names the chunk of a route file like `$flowId.tsx` "_flowId".
//
//   node scripts/check-embed-names.mjs [dist-folder]

import { readdirSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');

/** Paths (relative to the build folder) of the files go:embed would skip, apart from the `.vite` folder of the build tool. */
export function skippedByEmbed(distFolder) {
  const found = [];
  const visit = (folder) => {
    for (const entry of readdirSync(folder, { withFileTypes: true })) {
      const path = join(folder, entry.name);
      const name = relative(distFolder, path);
      if (name === '.vite') continue;
      if (/^[._]/.test(entry.name)) found.push(name);
      else if (entry.isDirectory()) visit(path);
    }
  };
  visit(distFolder);
  return found;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const dist = resolve(process.argv[2] ?? join(root, 'dist'));
  const skipped = skippedByEmbed(dist);
  if (skipped.length > 0) {
    console.error('check-embed-names: the binary would not embed these files (go:embed skips names that start with "_" or "."):');
    for (const name of skipped) console.error(`  ${name}`);
    process.exit(1);
  }
  console.log('check-embed-names ok: every file of the build can be embedded');
}
