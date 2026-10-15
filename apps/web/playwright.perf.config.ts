// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { spawnSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { existsSync, mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
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
 * A first start of the binary on an empty data directory refuses to run until the operator confirms
 * the backup of the master key by its fingerprint (keys.master.backupAcknowledgement). The perf run
 * makes a throwaway key on a fresh data directory, reads the fingerprint from that refusal and passes
 * it on, the same way scripts/smoke.sh does. The key only lives in this temporary directory.
 */
function prepareFirstStart(): { dataDir: string; backupAcknowledgement: string } {
  const dataDir = mkdtempSync(join(tmpdir(), 'orcon-perf-'));
  mkdirSync(join(dataDir, 'keys'));
  writeFileSync(join(dataDir, 'keys', 'master.key'), `${randomBytes(32).toString('base64')}\n`, { mode: 0o600 });

  const env: NodeJS.ProcessEnv = { ...process.env, ORCON_DATA_DIR: dataDir };
  delete env['ORCON_MASTER_KEY_BACKUP_ACK'];
  const refusal = spawnSync(binary, ['serve', '--role=standalone', `--http-address=127.0.0.1:${String(appPort)}`], {
    env,
    encoding: 'utf8',
    timeout: 30_000,
  });
  const said = `${refusal.stdout}${refusal.stderr}`;
  const fingerprint = /master key fingerprint ([0-9a-f]+)/.exec(said);
  if (!fingerprint) {
    throw new Error(`The first start did not name the master key fingerprint; it said: ${said}`);
  }
  return { dataDir, backupAcknowledgement: fingerprint[1] ?? '' };
}

const firstStart = staticServer ? undefined : prepareFirstStart();

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
      : {
          command: `${binary} serve --role=standalone --http-address=127.0.0.1:${String(appPort)}`,
          env: { ORCON_DATA_DIR: firstStart?.dataDir ?? '', ORCON_MASTER_KEY_BACKUP_ACK: firstStart?.backupAcknowledgement ?? '' },
          url: `http://127.0.0.1:${String(appPort)}/healthz`,
          timeout: 30_000,
          reuseExistingServer: false,
        },
    {
      command: `pnpm exec vite preview --config vite.perf.config.ts --host 127.0.0.1 --port ${String(editorPort)} --strictPort`,
      url: `http://127.0.0.1:${String(editorPort)}/`,
      timeout: 30_000,
      reuseExistingServer: false,
    },
  ],
});
