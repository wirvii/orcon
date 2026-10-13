// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// The initial JavaScript budget (orcon-web PF-1): the entry chunk and every chunk it imports
// statically, as the Vite manifest lists them. Chunks loaded on demand (route code, the editor)
// do not count. The limit is 250,000 bytes of gzip.

import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { initialJavaScript } from './scripts/initial-chunks.mjs';

const dist = join(dirname(fileURLToPath(import.meta.url)), 'dist');

export default [
  {
    name: 'initial JS',
    path: initialJavaScript(dist).map((file) => join(dist, file)),
    gzip: true,
    limit: '250 kB',
  },
];
