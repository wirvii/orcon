// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from 'vitest/config';

// Not derived from vite.config.ts: tests do not need the router plugin or Tailwind.
export default defineConfig({
  test: {
    projects: [
      {
        // Pure logic in a Node environment: models, boundaries, the event dispatcher, key factories.
        test: {
          name: 'unit',
          environment: 'node',
          include: ['src/**/*.test.ts'],
        },
      },
    ],
  },
});
