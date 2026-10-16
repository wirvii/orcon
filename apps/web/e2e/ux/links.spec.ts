// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test, type Page } from '@playwright/test';

import { screens } from './screens';

// No link is underlined outside running text (SPEC-019 R-UX-GEN-05, D8). A link in a paragraph is underlined so it
// can be told from the words around it; a link that is a destination, a row, a menu entry or an action is not. The
// check reads the style the browser really computed, not the class names.

/** The links that are underlined at rest and are not inside a container that declares running text. */
async function underlinedLinks(page: Page): Promise<string[]> {
  return page.evaluate(() =>
    Array.from(document.querySelectorAll<HTMLAnchorElement>('a'))
      .filter((link) => link.closest('[data-prose]') === null)
      .filter((link) => getComputedStyle(link).textDecorationLine.includes('underline'))
      .map((link) => `${link.textContent.trim().slice(0, 40)} (${link.getAttribute('href') ?? 'no href'})`),
  );
}

const titles: Record<string, string> = {
  'sign-in': 'no underlined link outside prose on sign-in',
  overview: 'no underlined link outside prose on overview',
  project: 'no underlined link outside prose on project',
  executions: 'no underlined link outside prose on executions',
  credentials: 'no underlined link outside prose on credentials',
  settings: 'no underlined link outside prose on settings',
  editor: 'no underlined link outside prose on editor',
};

for (const screen of screens) {
  test(titles[screen.name] ?? `no underlined link outside prose on ${screen.name}`, async ({ page }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem('orcon.locale', 'en');
    });
    await page.setViewportSize({ width: 1440, height: 900 });
    await screen.open(page);
    // There are links to look at: a page without any would pass for the wrong reason.
    expect(await page.locator('a').count()).toBeGreaterThan(0);
    expect(await underlinedLinks(page)).toEqual([]);
  });
}

test('the check reports an underlined link outside running text', async ({ page }) => {
  await page.setContent('<a href="#x" style="text-decoration-line: underline">a destination</a>');
  expect(await underlinedLinks(page)).toEqual(['a destination (#x)']);
});

test('the check leaves a link inside running text alone', async ({ page }) => {
  await page.setContent('<p data-prose>Read the <a href="#x" style="text-decoration-line: underline">notes</a>.</p>');
  expect(await underlinedLinks(page)).toEqual([]);
});
