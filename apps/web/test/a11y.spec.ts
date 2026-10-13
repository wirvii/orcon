// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

// WCAG 2.0 to 2.2, levels A and AA.
const wcag = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

const viewports = [
  { name: 'phone', width: 390, height: 844 },
  { name: 'desktop', width: 1440, height: 900 },
] as const;

for (const theme of ['light', 'dark'] as const) {
  for (const viewport of viewports) {
    test(`the welcome screen has no accessibility violations in the ${theme} theme on a ${viewport.name}`, async ({ page }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      // The saved preference is what theme-init.js applies before the first paint.
      await page.addInitScript((value) => window.localStorage.setItem('orcon.theme', value), theme);
      await page.goto('/');

      await expect(page.locator('#root')).toBeVisible();
      await expect(page.getByRole('heading', { level: 1, name: 'Welcome to Orcon' })).toBeVisible();
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme);

      const results = await new AxeBuilder({ page }).withTags(wcag).analyze();
      const report = results.violations.map((violation) => `${violation.id}: ${violation.help} (${violation.nodes.map((node) => node.target.join(' ')).join(', ')})`);
      expect(report).toEqual([]);
    });
  }
}

test('the page asks nothing from another origin', async ({ page }) => {
  const external: string[] = [];
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (url.protocol.startsWith('http') && url.origin !== 'http://127.0.0.1:4173') external.push(request.url());
  });
  await page.goto('/');
  await expect(page.locator('#root')).toBeVisible();
  expect(external).toEqual([]);
});

test('the saved density and language are on the page before it paints', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('orcon.density', 'compact');
    window.localStorage.setItem('orcon.locale', 'es');
  });
  await page.goto('/');
  await expect(page.locator('html')).toHaveAttribute('data-density', 'compact');
  await expect(page.locator('html')).toHaveAttribute('lang', 'es');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Te damos la bienvenida a Orcon');
});
