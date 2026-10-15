// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from '@playwright/test';

// Browser checks of the screens the Go binary serves. They run against the production build (`vite preview`), so
// what is tested is what ships. `test/` holds the page-level checks; `e2e/ux/` holds the checks of the redesigned
// screens (SPEC-019). The captures for a pull request (`e2e/ux/capture.spec.ts`) are evidence, not checks, and run
// only through `playwright.ux-review.config.ts`.
export default defineConfig({
  testDir: '.',
  testMatch: ['test/**/*.spec.ts', 'e2e/ux/**/*.spec.ts'],
  testIgnore: ['e2e/ux/capture.spec.ts'],
  fullyParallel: true,
  reporter: 'list',
  use: {
    baseURL: 'http://127.0.0.1:4173',
    locale: 'en-US',
    timezoneId: 'UTC',
    reducedMotion: 'reduce',
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
  webServer: {
    command: 'pnpm exec vite preview --host 127.0.0.1 --port 4173 --strictPort',
    url: 'http://127.0.0.1:4173',
    reuseExistingServer: !process.env['CI'],
    timeout: 60_000,
  },
});
