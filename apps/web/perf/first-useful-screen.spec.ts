// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test';

import { median, recordResult } from './results';

// How these numbers were taken (orcon-web PF-3, maintainer decision of 2026-10-12): the phone profile
// is an emulation in Chromium on the CI runner, an approximation and not a physical phone.
const method = 'emulated-approximation';

const LOADS = 5;
/** Budgets in milliseconds, by profile (orcon-web PF-3). */
const budgets = { desktop: 1500, phone: 3000 } as const;
/** The routes with a first-useful-screen mark. Every main route joins this list when it is built. */
const routes = ['/login', '/settings/appearance', '/flows/01930000-0000-7000-8000-00000000ed5a'] as const;

type Profile = keyof typeof budgets;

/** The emulated phone: CPU 4x slower, 150 ms latency, 1.6 Mbps down and 750 Kbps up. */
const phone = {
  cpuSlowdown: 4,
  latencyMs: 150,
  downloadBytesPerSecond: (1.6 * 1000 * 1000) / 8,
  uploadBytesPerSecond: (750 * 1000) / 8,
} as const;

function profileOf(metadata: unknown): Profile {
  const profile = typeof metadata === 'object' && metadata !== null && 'profile' in metadata ? metadata.profile : undefined;
  if (profile === 'desktop' || profile === 'phone') return profile;
  throw new Error('The project declares no performance profile (desktop or phone).');
}

test('the first useful screen of every main route', async ({ browser }, testInfo) => {
  const profile = profileOf(testInfo.project.metadata);
  const baseURL = testInfo.project.use.baseURL;
  if (!baseURL) throw new Error('The project has no baseURL.');
  const results: Record<string, { samples: number[]; median: number }> = {};

  for (const route of routes) {
    const samples: number[] = [];
    for (let load = 0; load < LOADS; load += 1) {
      // A new context is a cold start: empty cache, no service worker, nothing saved.
      const context = await browser.newContext({
        viewport: testInfo.project.use.viewport ?? null,
        isMobile: testInfo.project.use.isMobile ?? false,
        hasTouch: testInfo.project.use.hasTouch ?? false,
        locale: 'en-US',
        timezoneId: 'UTC',
      });
      const page = await context.newPage();
      if (profile === 'phone') {
        const session = await context.newCDPSession(page);
        await session.send('Emulation.setCPUThrottlingRate', { rate: phone.cpuSlowdown });
        await session.send('Network.emulateNetworkConditions', {
          offline: false,
          latency: phone.latencyMs,
          downloadThroughput: phone.downloadBytesPerSecond,
          uploadThroughput: phone.uploadBytesPerSecond,
        });
      }
      await page.goto(new URL(route, baseURL).href, { waitUntil: 'commit' });
      await page.waitForFunction(() => performance.getEntriesByName('orcon:first-useful').length > 0, undefined, { timeout: 60_000 });
      const milliseconds = await page.evaluate(() => performance.getEntriesByName('orcon:first-useful')[0]?.startTime ?? Number.NaN);
      expect(Number.isFinite(milliseconds), `${route}: the route never marked the first useful screen`).toBe(true);
      samples.push(milliseconds);
      await context.close();
    }
    results[route] = { samples, median: median(samples) };
  }

  recordResult(`first-useful-screen-${profile}`, {
    method,
    profile,
    budgetMs: budgets[profile],
    ...(profile === 'phone' ? { emulation: phone } : {}),
    loads: LOADS,
    routes: results,
  });

  for (const route of routes) {
    expect(results[route]?.median ?? Number.POSITIVE_INFINITY, `${route} on the ${profile} profile (median of ${String(LOADS)} cold loads)`).toBeLessThan(budgets[profile]);
  }
});
