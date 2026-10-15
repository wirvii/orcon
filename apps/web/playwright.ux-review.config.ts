// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from '@playwright/test';

// Captures of the screens for a pull request (e2e/ux/capture.spec.ts). Evidence, not a check: it is not part of
// `make check` or `make web-e2e`. It runs against the production build served by `vite preview`.
export default defineConfig({
  testDir: 'e2e/ux',
  testMatch: 'capture.spec.ts',
  fullyParallel: true,
  reporter: 'list',
  use: {
    baseURL: 'http://127.0.0.1:4173',
    locale: 'es-CO',
    timezoneId: 'America/Bogota',
    reducedMotion: 'reduce',
  },
  projects: [{ name: 'ux-review', use: { browserName: 'chromium' } }],
  webServer: {
    command: 'pnpm exec vite preview --host 127.0.0.1 --port 4173 --strictPort',
    url: 'http://127.0.0.1:4173',
    reuseExistingServer: !process.env['CI'],
    timeout: 60_000,
  },
});
