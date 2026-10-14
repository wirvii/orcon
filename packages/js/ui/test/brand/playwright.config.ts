// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from '@playwright/test';

const PORT = 4178;

// Brand fidelity: the review page measures how closely each vector matches its
// region of the approved board; fidelity.spec.ts reads those measurements.
export default defineConfig({
  testDir: '.',
  testMatch: '*.spec.ts',
  outputDir: './test-results',
  fullyParallel: false,
  retries: 0,
  reporter: 'list',
  timeout: 120_000,
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    browserName: 'chromium',
    deviceScaleFactor: 1,
  },
  webServer: {
    // The config directory is packages/js/ui/test/brand, five levels below the repository root.
    command: 'node ../../../../../brand/review/serve.mjs',
    url: `http://127.0.0.1:${PORT}/brand/review/`,
    reuseExistingServer: !process.env['CI'],
    timeout: 30_000,
  },
});
