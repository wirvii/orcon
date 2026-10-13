// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

import { createConfig } from '@orcon/lint-config';
import { createWebAppConfig } from '@orcon/lint-config/web-app';

const here = dirname(fileURLToPath(import.meta.url));

export default [
  ...createConfig(),
  ...createWebAppConfig({ tsconfigRootDir: here }),
  {
    // This package is where raw color values are defined and asserted.
    files: ['**/*.{ts,mjs}'],
    rules: { 'orcon/no-color-literal': 'off' },
  },
];
