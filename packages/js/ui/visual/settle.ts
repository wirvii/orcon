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
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
  });
}
