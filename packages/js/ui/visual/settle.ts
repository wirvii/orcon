// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Page } from '@playwright/test';

/**
 * Waits until a story is drawn and stable: the story root has content, fonts are loaded, React
 * Flow has measured its nodes, and two frames have passed.
 */
export async function settle(page: Page): Promise<void> {
  await page.waitForSelector('body.sb-show-main', { timeout: 20_000 });
  await page.evaluate(async () => {
    await document.fonts.ready;
    const root = document.querySelector('#storybook-root');
    if (!root || root.children.length === 0) throw new Error('The story rendered nothing.');
    // React Flow hides a node until it has measured it.
    const started = performance.now();
    while (performance.now() - started < 5000) {
      const nodes = Array.from(document.querySelectorAll<HTMLElement>('.react-flow__node'));
      if (nodes.every((node) => getComputedStyle(node).visibility !== 'hidden')) break;
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    // Every canvas has applied its first fit, and its viewport has stopped moving.
    const marks = () => Array.from(document.querySelectorAll<HTMLElement>('[data-lod]'));
    const viewportOf = () => marks().map((region) => region.querySelector<HTMLElement>('.react-flow__viewport')?.style.transform ?? '').join('|');
    const steadyStart = performance.now();
    let last = '';
    let steady = 0;
    while (performance.now() - steadyStart < 10_000 && steady < 3) {
      // The fit was made for the size the canvas has now: other content (text that wraps once the
      // fonts are drawn) can resize it after the first fit, and the canvas fits again when it does.
      const fitted = marks().every((region) => {
        const box = region.getBoundingClientRect();
        const size = region.getAttribute('data-fit-size');
        // A canvas without nodes has nothing to fit and carries no size.
        return region.hasAttribute('data-fitted') && (size === null || size === `${Math.round(box.width)}x${Math.round(box.height)}`);
      });
      const now = viewportOf();
      steady = fitted && now === last ? steady + 1 : 0;
      last = now;
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
  });
}
