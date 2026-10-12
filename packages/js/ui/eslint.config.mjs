// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { createConfig } from '@orcon/lint-config';

const here = dirname(fileURLToPath(import.meta.url));

export default [
  ...createConfig({
    // Reports Tailwind classes the Orcon theme does not define.
    tailwindEntryPoint: resolve(here, 'src/styles/index.css'),
    cwd: here,
  }),
];
