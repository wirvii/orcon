// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { createConfig } from '@orcon/lint-config';
import { createWebAppConfig } from '@orcon/lint-config/web-app';

const here = dirname(fileURLToPath(import.meta.url));

export default [
  ...createConfig({
    // Reports Tailwind classes the Orcon theme does not define.
    tailwindEntryPoint: resolve(here, 'src/styles/index.css'),
    cwd: here,
  }),
  ...createWebAppConfig({ tsconfigRootDir: here, untypedFiles: ['.storybook/main.ts'] }),
  {
    // The brand art classes (theme choice, mascot and hero sizes) are defined in src/styles/index.css, outside the Tailwind theme.
    // React Flow marks parts of a node that must not start a drag, a pan or a wheel zoom with these classes.
    files: ['**/*.{ts,tsx}'],
    rules: { 'better-tailwindcss/no-unknown-classes': ['error', { ignore: ['^(nodrag|nopan|nowheel)$', '^orcon-(edge-focus|run-ring|handle|handle-in)$', '^orcon-(brand-on-dark|brand-on-light|brand-hero-image|mascot-image)$'] }] },
  },
  {
    // Stories force interaction states with the pseudo-* classes of the Storybook pseudo-states addon.
    files: ['**/*.stories.tsx'],
    rules: { 'better-tailwindcss/no-unknown-classes': ['error', { ignore: ['^pseudo-(hover|focus|focus-visible|active)$', '^(nodrag|nopan|nowheel)$', '^orcon-(edge-focus|run-ring|handle|handle-in)$', '^orcon-(brand-on-dark|brand-on-light|brand-hero-image|mascot-image)$'] }] },
  },
];
