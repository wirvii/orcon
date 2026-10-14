// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { existsSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { defineConfig } from '@playwright/test';

const here = dirname(fileURLToPath(import.meta.url));
const binary = resolve(here, '../../bin/orcon');
const appPort = 18081;
const editorPort = 4174;

// The first-useful-screen budget is measured against the real binary (`make build` first). Where the
// binary is not built, ORCON_PERF_SERVER=static serves the same build from a small compressed server.
const staticServer = process.env['ORCON_PERF_SERVER'] === 'static';
if (!staticServer && !existsSync(binary)) {
  throw new Error(`The binary ${binary} is not built. Run \`make build\`, or set ORCON_PERF_SERVER=static to serve the build without it.`);
}

/**
 * Performance measurements (orcon-web PF-3, PF-4). They run one at a time and with a cold cache
 * on the CI runner. The phone profile is an emulation in Chromium (CPU 4x slower, 150 ms latency,
 * 1.6 Mbps down, 750 Kbps up): a reproducible approximation of a mid-range phone on 4G, not a
 * measurement on a physical phone (maintainer decision, 2026-10-12).
 */
export default defineConfig({
  testDir: 'perf',
  testMatch: '*.spec.ts',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 180_000,
  reporter: 'list',
  use: { locale: 'en-US', timezoneId: 'UTC', reducedMotion: 'no-preference', trace: 'retain-on-failure' },
  projects: [
    {
      name: 'perf-desktop',
      use: { browserName: 'chromium', viewport: { width: 1440, height: 900 }, baseURL: `http://127.0.0.1:${String(appPort)}` },
      metadata: { profile: 'desktop', editorURL: `http://127.0.0.1:${String(editorPort)}` },
    },
    {
      name: 'perf-phone',
      testMatch: 'first-useful-screen.spec.ts',
      use: {
        browserName: 'chromium',
        viewport: { width: 390, height: 844 },
        isMobile: true,
        hasTouch: true,
        baseURL: `http://127.0.0.1:${String(appPort)}`,
      },
      metadata: { profile: 'phone' },
    },
  ],
  webServer: [
    staticServer
      ? { command: `node perf/serve-static.mjs ${String(appPort)}`, url: `http://127.0.0.1:${String(appPort)}/healthz`, timeout: 30_000, reuseExistingServer: false }
      : { command: `${binary} --role=standalone --addr=127.0.0.1:${String(appPort)}`, url: `http://127.0.0.1:${String(appPort)}/healthz`, timeout: 30_000, reuseExistingServer: false },
    {
      command: `pnpm exec vite preview --config vite.perf.config.ts --host 127.0.0.1 --port ${String(editorPort)} --strictPort`,
      url: `http://127.0.0.1:${String(editorPort)}/`,
      timeout: 30_000,
      reuseExistingServer: false,
    },
  ],
});
