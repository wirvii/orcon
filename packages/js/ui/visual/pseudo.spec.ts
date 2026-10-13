// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test';

import { settle } from './settle';
import { loadTargets, storyUrl, viewports } from './targets';

// In the long-string pseudo-locale (xx-LS, text well over 30 % longer) no text container may
// overflow, unless truncation is declared with data-truncate (and a tooltip carries the text).
test.describe('pseudo-locale xx-LS', () => {
  for (const target of loadTargets()) {
    for (const viewport of [viewports.mobile, viewports.desktop]) {
      test(`${target.title} ${target.name} on ${viewport.name}`, async ({ page }) => {
        await page.setViewportSize({ width: viewport.width, height: viewport.height });
        await page.goto(storyUrl(target.id, { theme: 'light', density: 'comfortable', locale: 'xx-LS' }));
        await settle(page);

        const overflowing = await page.evaluate(() => {
          const found: string[] = [];
          for (const element of document.querySelectorAll<HTMLElement>('#storybook-root *')) {
            if (element.closest('[data-truncate]') || element.closest('[aria-hidden="true"]')) continue;
            if (!Array.from(element.childNodes).some((node) => node.nodeType === Node.TEXT_NODE && node.textContent?.trim())) continue;
            const style = getComputedStyle(element);
            // A region that scrolls on purpose (code block, table region) is a declared scroller.
            if (style.overflowX === 'auto' || style.overflowX === 'scroll') continue;
            // Visually hidden text is for screen readers and is not drawn.
            if (element.clientWidth <= 1 && element.clientHeight <= 1) continue;
            if (element.scrollWidth > element.clientWidth + 1 && element.clientWidth > 0) {
              found.push(`${element.tagName.toLowerCase()} "${element.textContent?.trim().slice(0, 40)}" (${element.scrollWidth} > ${element.clientWidth})`);
            }
          }
          return found;
        });
        expect(overflowing).toEqual([]);
      });
    }
  }
});
