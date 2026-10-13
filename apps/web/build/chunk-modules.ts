// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { relative } from 'node:path';

import type { Plugin } from 'vite';

/** What `.vite/chunk-modules.json` holds for one JavaScript chunk. */
export interface ChunkModules {
  /** File name inside the output folder: `assets/index-abc.js`. */
  file: string;
  isEntry: boolean;
  /** The modules rolled into the chunk, as paths from the repository root; `\0` virtual modules are dropped. */
  modules: string[];
}

/**
 * Writes `.vite/chunk-modules.json` next to the Vite manifest: which modules ended up in which
 * chunk. The manifest lists chunks and their imports but not their contents, and the bundle
 * checks (`scripts/check-initial-chunks.mjs`) need to know that the editor is not in the initial
 * chunks (orcon-web PF-2).
 */
export function chunkModules(repositoryRoot: string): Plugin {
  return {
    name: 'orcon:chunk-modules',
    apply: 'build',
    generateBundle(_options, bundle) {
      const chunks: ChunkModules[] = Object.values(bundle).flatMap((item) =>
        item.type === 'chunk'
          ? [
              {
                file: item.fileName,
                isEntry: item.isEntry,
                modules: Object.keys(item.modules)
                  .filter((id) => !id.startsWith('\0'))
                  .map((id) => relative(repositoryRoot, id.split('?')[0] ?? id))
                  .sort(),
              },
            ]
          : [],
      );
      this.emitFile({ type: 'asset', fileName: '.vite/chunk-modules.json', source: `${JSON.stringify(chunks, null, 2)}\n` });
    },
  };
}
