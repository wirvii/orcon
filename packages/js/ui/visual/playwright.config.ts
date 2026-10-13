// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from '@playwright/test';

const PORT = 6199;

// Three projects share the built Storybook:
//   visual  every Matrix and screen template, compared with the committed baselines
//   parity  the end state of every Editor/Parity story, in light and dark
//   pseudo  the same targets in the long-string pseudo-locale, checked for text that overflows
// Baselines are only valid when made in the pinned Playwright container (see run-docker.sh):
// fonts and rasterization differ per operating system.
export default defineConfig({
  testDir: '.',
  testMatch: '*.spec.ts',
  snapshotPathTemplate: '{testDir}/__screenshots__/{arg}{ext}',
  outputDir: './test-results',
  fullyParallel: true,
  ...(process.env['CI'] ? { workers: 2 } : {}),
  retries: 0,
  reporter: process.env['CI'] ? [['list'], ['html', { open: 'never', outputFolder: './report' }]] : 'list',
  expect: { toHaveScreenshot: { maxDiffPixelRatio: 0.001, animations: 'disabled', caret: 'hide' } },
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    locale: 'en-US',
    timezoneId: 'UTC',
    reducedMotion: 'reduce',
    colorScheme: 'light',
    deviceScaleFactor: 1,
  },
  projects: [
    { name: 'visual', testMatch: 'visual.spec.ts', use: { browserName: 'chromium' } },
    { name: 'parity', testMatch: 'parity.spec.ts', use: { browserName: 'chromium' } },
    { name: 'pseudo', testMatch: 'pseudo.spec.ts', use: { browserName: 'chromium' } },
  ],
  webServer: {
    command: `node visual/serve.mjs ${PORT}`,
    cwd: '..',
    url: `http://127.0.0.1:${PORT}/index.json`,
    reuseExistingServer: !process.env['CI'],
    timeout: 30_000,
  },
});
