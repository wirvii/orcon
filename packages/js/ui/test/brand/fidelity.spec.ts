// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test';

// Floors of SPEC-018 (R-BR-V-03). A manifest floor lower than these is a failure.
const FLOORS: Record<string, number> = {
  symbol: 0.95,
  'app icon': 0.95,
  'wordmark on dark': 0.93,
  'wordmark on light': 0.93,
  'lockup on dark': 0.93,
  'lockup on light': 0.93,
  'descriptor lockup on dark': 0.9,
  'descriptor lockup on light': 0.9,
  'descriptor on dark': 0.85,
  'descriptor on light': 0.85,
};

for (const name of Object.keys(FLOORS)) {
  test(`${name} matches the board`, async ({ page }) => {
    await page.goto('/brand/review/');
    await page.waitForFunction(() => document.body.dataset['ready'] !== undefined);
    expect(await page.evaluate(() => document.body.dataset['ready'])).toBe('true');
    const card = page.locator(`[data-fidelity="${name}"]`);
    await expect(card).toHaveCount(1);
    const iou = Number(await card.getAttribute('data-iou'));
    const min = Number(await card.getAttribute('data-iou-min'));
    expect(min).toBeGreaterThan(0);
    expect(iou).toBeGreaterThanOrEqual(min);
  });
}

test('the floors are not lower than the specification', async ({ request }) => {
  const response = await request.get('/brand/manifest.json');
  expect(response.ok()).toBe(true);
  const manifest = (await response.json()) as { assets: { fidelity?: { name: string; iou_min: number }[] }[] };
  const found = new Map<string, number>();
  for (const asset of manifest.assets) {
    for (const f of asset.fidelity ?? []) found.set(f.name, f.iou_min);
  }
  expect([...found.keys()].sort()).toEqual(Object.keys(FLOORS).sort());
  for (const [name, floor] of Object.entries(FLOORS)) {
    expect(found.get(name), name).toBeGreaterThanOrEqual(floor);
  }
});
