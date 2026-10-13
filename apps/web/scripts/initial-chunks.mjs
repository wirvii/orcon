// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs';
import { join } from 'node:path';

/**
 * The files of the entry chunk and of every chunk it imports statically, from the Vite manifest
 * (`<dist>/.vite/manifest.json`). Chunks loaded on demand (`dynamicImports`) are not initial.
 */
export function initialJavaScript(distFolder) {
  const manifestPath = join(distFolder, '.vite', 'manifest.json');
  const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
  const entry = Object.values(manifest).find((chunk) => chunk.isEntry && chunk.src === 'index.html');
  if (!entry) throw new Error(`${manifestPath} has no entry chunk for index.html. Run the build first.`);
  const files = new Set([entry.file]);
  const visit = (key) => {
    const chunk = manifest[key];
    if (!chunk || files.has(chunk.file)) return;
    files.add(chunk.file);
    for (const imported of chunk.imports ?? []) visit(imported);
  };
  for (const imported of entry.imports ?? []) visit(imported);
  return [...files].filter((file) => file.endsWith('.js'));
}
