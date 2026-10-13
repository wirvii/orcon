// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { expect, test } from '@playwright/test';

import { settle } from './settle';
import { storyUrl, themes } from './targets';

// The end state of every parity row (the stories of "Editor/Parity"), in light and dark. Each story
// plays its keyboard script and marks the page when it is done; the screenshot is taken after that.
// Baselines come from the pinned container only (see run-docker.sh and the visual-baselines workflow).
const here = dirname(fileURLToPath(import.meta.url));
const index = JSON.parse(readFileSync(resolve(here, '../storybook-static/index.json'), 'utf8')) as {
  entries: Record<string, { id: string; type: string; title: string }>;
};
const stories = Object.values(index.entries)
  .filter((entry) => entry.type === 'story' && entry.title === 'Editor/Parity')
  .map((entry) => entry.id)
  .sort();

for (const id of stories) {
  for (const theme of themes) {
    test(`${id} ${theme}`, async ({ page }) => {
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(storyUrl(id, { theme, density: 'comfortable' }));
      await settle(page);
      await page.waitForSelector('#storybook-root[data-parity="done"]', { timeout: 30_000 });
      // Let transitions and the minimap fade settle (motion is reduced, so this is short).
      await page.waitForTimeout(300);
      await expect(page).toHaveScreenshot(`${id}-${theme}.png`);
    });
  }
}
