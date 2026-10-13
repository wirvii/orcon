// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Fails when the flow editor leaks into the initial JavaScript (orcon-web PF-2): the entry chunk
// and the chunks it imports statically must hold no module of @xyflow/react, zustand, immer or
// src/features/editor. The editor loads only through route code splitting.
//
//   node scripts/check-initial-chunks.mjs [dist-folder]

import { readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { initialJavaScript } from './initial-chunks.mjs';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const dist = resolve(process.argv[2] ?? join(root, 'dist'));

/** Substrings of a module path that mark the editor and its tools. */
export const forbidden = ['/node_modules/@xyflow/', '/node_modules/zustand/', '/node_modules/immer/', 'src/features/editor/'];

/** The forbidden modules inside the initial chunks: [{ file, module }]. */
export function leaks(distFolder) {
  const initial = new Set(initialJavaScript(distFolder));
  const chunks = JSON.parse(readFileSync(join(distFolder, '.vite', 'chunk-modules.json'), 'utf8'));
  return chunks
    .filter((chunk) => initial.has(chunk.file))
    .flatMap((chunk) => chunk.modules.filter((module) => forbidden.some((part) => module.includes(part))).map((module) => ({ file: chunk.file, module })));
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const found = leaks(dist);
  if (found.length > 0) {
    console.error('check-initial-chunks: the editor is in the initial JavaScript. Load it through a route or lazy():');
    for (const { file, module } of found) console.error(`  ${file}  contains  ${module}`);
    process.exit(1);
  }
  console.log(`check-initial-chunks ok: ${initialJavaScript(dist).length} initial chunk(s), no @xyflow/react, zustand, immer or editor module`);
}
