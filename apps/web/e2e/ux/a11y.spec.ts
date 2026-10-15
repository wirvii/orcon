// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

import { screens } from './screens';

// Every screen of the product, in both themes and at the two reference sizes, has no accessibility violation
// (WCAG 2.0 to 2.2, levels A and AA; SPEC-019 R-UX-GEN-10).

const wcag = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

const viewports = [
  { name: '1440x900', width: 1440, height: 900 },
  { name: '390x844', width: 390, height: 844 },
] as const;

for (const screen of screens) {
  for (const theme of ['dark', 'light'] as const) {
    for (const viewport of viewports) {
      test(`${screen.name} has no accessibility violations in the ${theme} theme at ${viewport.name}`, async ({ page }) => {
        await page.addInitScript(
          ({ value }) => {
            window.localStorage.setItem('orcon.theme', value);
            window.localStorage.setItem('orcon.locale', 'en');
          },
          { value: theme },
        );
        await page.setViewportSize({ width: viewport.width, height: viewport.height });
        await screen.open(page);
        await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
        const results = await new AxeBuilder({ page }).withTags(wcag).analyze();
        const report = results.violations.map((violation) => `${violation.id}: ${violation.help} (${violation.nodes.map((node) => node.target.join(' ')).join(', ')})`);
        expect(report).toEqual([]);
      });
    }
  }
}
