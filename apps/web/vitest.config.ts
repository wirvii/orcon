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
      {
        // Screens in jsdom with Testing Library, against Mock Service Worker: every request needs a handler.
        test: {
          name: 'screens',
          environment: 'jsdom',
          include: ['src/**/*.test.tsx'],
          setupFiles: ['test/setup-screens.ts'],
          css: false,
        },
      },
    ],
    coverage: {
      provider: 'v8',
      // The logic of the app: models, stores and data layers of every feature, and the runtime of app/.
      include: ['src/features/*/{model,store,api}/**/*.{ts,tsx}', 'src/app/{boundary,live-events,errors,forms,perf}/**/*.{ts,tsx}'],
      exclude: ['**/*.test.{ts,tsx}', '**/*.gen.ts', '**/gen/**'],
      reporter: ['text-summary', 'json-summary'],
      thresholds: { lines: 80, branches: 80, functions: 80, statements: 80 },
    },
  },
});
