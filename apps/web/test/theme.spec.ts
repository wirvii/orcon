// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test, type Page } from '@playwright/test';
import { z } from 'zod';

// The saved theme choice must survive a reload and be on <html> before the first paint. Each test
// opens the page on a device whose own setting is the opposite of the choice, so a choice that
// quietly falls back to the device setting shows up as the wrong colours.

type Shown = 'light' | 'dark';

const SETTINGS = '/settings/appearance';

/** What the page shows now: the attribute and the stored choice, and the colours the body really has. */
async function shown(page: Page) {
  return page.evaluate(() => {
    const style = window.getComputedStyle(document.body);
    return {
      attribute: document.documentElement.getAttribute('data-theme'),
      stored: window.localStorage.getItem('orcon.theme'),
      background: style.backgroundColor,
      text: style.color,
      scheme: window.getComputedStyle(document.documentElement).colorScheme,
    };
  });
}

/** The page and text colours of the design tokens for one theme, resolved by the browser itself. */
async function tokenColours(page: Page, theme: Shown) {
  return page.evaluate((name) => {
    const probe = document.createElement('div');
    probe.setAttribute('data-theme', name);
    probe.style.backgroundColor = 'var(--orcon-bg-app)';
    probe.style.color = 'var(--orcon-fg-default)';
    document.body.append(probe);
    const style = window.getComputedStyle(probe);
    const result = { background: style.backgroundColor, text: style.color };
    probe.remove();
    return result;
  }, theme);
}

async function save(page: Page, choice: 'System' | 'Light' | 'Dark') {
  await page.getByRole('radio', { name: choice, exact: true }).click();
  await page.getByRole('button', { name: 'Save preferences' }).click();
  await expect(page.getByText('Preferences saved')).toBeVisible();
}

const cases = [
  { choice: 'Light', attribute: 'light', stored: 'light' },
  { choice: 'Dark', attribute: 'dark', stored: 'dark' },
  { choice: 'System', attribute: null, stored: 'system' },
] as const;

for (const device of ['light', 'dark'] as const) {
  for (const { choice, attribute, stored } of cases) {
    const expected: Shown = attribute ?? device;
    test(`saving ${choice} keeps it after a reload on a ${device} device`, async ({ browser }) => {
      const context = await browser.newContext({ colorScheme: device, locale: 'en-US' });
      const page = await context.newPage();
      await page.goto(SETTINGS);
      // Nothing is written for System on a first visit, so move away from it first.
      if (choice === 'System') await save(page, 'Dark');
      await save(page, choice);

      for (const moment of ['saved', 'reloaded', 'opened again'] as const) {
        if (moment === 'reloaded') await page.reload();
        if (moment === 'opened again') await page.goto(SETTINGS);
        await expect(page.locator('#root')).toBeVisible();
        const now = await shown(page);
        const colours = await tokenColours(page, expected);
        expect(now, moment).toMatchObject({ attribute, stored, scheme: expected, ...colours });
      }
      await context.close();
    });
  }
}

const seenSchema = z.array(z.object({ theme: z.string().nullable(), bodyExisted: z.boolean() }));

test('the saved theme is on the page before it paints', async ({ browser }) => {
  // The device is dark and the saved choice is light: any late correction would flash dark.
  const context = await browser.newContext({ colorScheme: 'dark', locale: 'en-US' });
  const page = await context.newPage();
  await page.goto(SETTINGS);
  await save(page, 'Light');

  // Runs before any page script. <html> does not exist yet, so it watches the whole document.
  await page.addInitScript(() => {
    const seen: { theme: string | null; bodyExisted: boolean }[] = [];
    Reflect.set(window, '__themeSeen', seen);
    new MutationObserver(() => {
      seen.push({ theme: document.documentElement.getAttribute('data-theme'), bodyExisted: document.querySelector('body') !== null });
    }).observe(document, { attributes: true, attributeFilter: ['data-theme'], subtree: true });
  });
  await page.reload();
  await expect(page.locator('#root')).toBeVisible();

  const seen = seenSchema.parse(await page.evaluate(() => Reflect.get(window, '__themeSeen') as unknown));
  // The first thing set is the saved choice, and it happens while the document head is still being read.
  expect(seen[0]).toEqual({ theme: 'light', bodyExisted: false });
  expect(seen.every((entry) => entry.theme === 'light')).toBe(true);
  await context.close();
});
