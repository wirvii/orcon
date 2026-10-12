// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createConfig } from '@orcon/lint-config';

export default [
  ...createConfig(),
  {
    // This package is where raw color values are defined and asserted.
    files: ['**/*.{ts,mjs}'],
    rules: { 'orcon/no-color-literal': 'off' },
  },
];
