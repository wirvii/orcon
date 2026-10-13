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
  {
    // React Flow marks parts of a node that must not start a drag, a pan or a wheel zoom with these classes.
    files: ['**/*.{ts,tsx}'],
    rules: { 'better-tailwindcss/no-unknown-classes': ['error', { ignore: ['^(nodrag|nopan|nowheel)$', '^orcon-(edge-focus|run-ring|handle|handle-in)$'] }] },
  },
  {
    // Stories force interaction states with the pseudo-* classes of the Storybook pseudo-states addon.
    files: ['**/*.stories.tsx'],
    rules: { 'better-tailwindcss/no-unknown-classes': ['error', { ignore: ['^pseudo-(hover|focus|focus-visible|active)$', '^(nodrag|nopan|nowheel)$', '^orcon-(edge-focus|run-ring|handle|handle-in)$'] }] },
  },
];
