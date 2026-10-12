// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test';

import { settle } from './settle';
import { densities, loadTargets, storyUrl, themes } from './targets';

// Every Matrix and every screen template, in light and dark, in comfortable and compact, on a
// phone and a desktop (and a tablet for the flow editor), compared with the committed baseline.
// Baselines come from the pinned container only: run `pnpm --filter @orcon/ui test:visual:docker`.
for (const target of loadTargets()) {
  test.describe(target.title, () => {
    for (const viewport of target.viewports) {
      for (const theme of themes) {
        for (const density of densities) {
          // The tablet is only required for the flow editor, in both themes and both densities.
          test(`${viewport.name} ${theme} ${density}`, async ({ page }) => {
            await page.setViewportSize({ width: viewport.width, height: viewport.height });
            await page.goto(storyUrl(target.id, { theme, density }));
            await settle(page);
            await expect(page).toHaveScreenshot(`${target.id}/${viewport.name}-${theme}-${density}.png`, { fullPage: true });
          });
        }
      }
    }
  });
}
