// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from '@playwright/test';

// The end-to-end journeys of the signed-in product (SPEC-016 SH8). Without a server they run against the
// production build and the stand-in API in e2e/journeys/api-double.ts. With ORCON_JOURNEY_BASE_URL they run
// against a real server, which must already be running, and no preview server is started.
const live = process.env['ORCON_JOURNEY_BASE_URL'];
const baseURL = live ?? 'http://127.0.0.1:4173';

export default defineConfig({
  testDir: 'e2e/journeys',
  // The journey creates a project and a flow in a row: its steps run in order, one page at a time.
  fullyParallel: false,
  workers: 1,
  reporter: 'list',
  use: {
    baseURL,
    locale: 'en-US',
    timezoneId: 'UTC',
    reducedMotion: 'reduce',
  },
  projects: [
    { name: 'desktop', use: { browserName: 'chromium', viewport: { width: 1440, height: 900 } } },
    { name: 'phone', use: { browserName: 'chromium', viewport: { width: 390, height: 844 } } },
  ],
  ...(live
    ? {}
    : {
        webServer: {
          command: 'pnpm exec vite preview --host 127.0.0.1 --port 4173 --strictPort',
          url: baseURL,
          reuseExistingServer: !process.env['CI'],
          timeout: 60_000,
        },
      }),
});
