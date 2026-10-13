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
  // The pipeline test generates into a scratch folder; it is not part of the package.
  { ignores: ['test/.out/**'] },
];
