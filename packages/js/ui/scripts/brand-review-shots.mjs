// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Saves one PNG per section of the brand comparison page (brand/review/) and
// one per card, so the owner can review without opening the page.
// Output directory: BRAND_REVIEW_OUT (default: a temporary directory, never the repository).

import { mkdirSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { chromium } from '@playwright/test';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const { createReviewServer } = await import(pathToFileURL(path.join(root, 'brand/review/serve.mjs')).href);

const out = process.env.BRAND_REVIEW_OUT ?? mkdtempSync(path.join(tmpdir(), 'orcon-brand-review-'));
mkdirSync(out, { recursive: true });
const zoom = process.env.BRAND_REVIEW_ZOOM ?? '2';

const server = createReviewServer();
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const { port } = server.address();
const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1500, height: 900 } });
  await page.goto(`http://127.0.0.1:${port}/brand/review/?zoom=${zoom}`);
  await page.waitForFunction(() => document.body.dataset.ready, null, { timeout: 120000 });
  if ((await page.evaluate(() => document.body.dataset.ready)) !== 'true') throw new Error('the review page failed to load');
  const names = new Map();
  const slug = (s) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
  let n = 0;
  for (const section of await page.locator('section[data-section]').all()) {
    const name = await section.getAttribute('data-section');
    const cards = await section.locator('article.card').all();
    if (cards.length === 0) {
      await section.screenshot({ path: path.join(out, `${String(++n).padStart(2, '0')}-${name}.png`) });
      console.log(path.join(out, `${String(n).padStart(2, '0')}-${name}.png`));
      continue;
    }
    for (const card of cards) {
      const uses = await card.getAttribute('data-uses');
      const label =
        (await card.getAttribute('data-fidelity')) ??
        (uses ? `sizes ${uses.replace(/^brand\/(logo|icon)\//, '')}` : null) ??
        (await card.getAttribute('data-mascot')) ??
        (await card.getAttribute('data-composite')) ??
        (await card.getAttribute('data-icons') ? 'favicon set' : null) ??
        'card';
      let file = `${String(++n).padStart(2, '0')}-${name}-${slug(label.replace(/^brand\//, ''))}.png`;
      if (names.has(file)) file = file.replace('.png', `-${n}.png`);
      names.set(file, true);
      await card.screenshot({ path: path.join(out, file) });
      console.log(path.join(out, file));
    }
  }
} finally {
  await browser.close();
  server.close();
}
