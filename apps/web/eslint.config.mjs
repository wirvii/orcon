// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

import { createConfig } from '@orcon/lint-config';

const here = dirname(fileURLToPath(import.meta.url));

export default [
  ...createConfig({ cwd: here }),
  {
    // The service worker free browser scripts under public/ run as plain scripts, not modules.
    files: ['public/**/*.js'],
    languageOptions: { sourceType: 'script' },
  },
];
