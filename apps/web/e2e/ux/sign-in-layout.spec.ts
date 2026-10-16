// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test, type Page } from '@playwright/test';

import { account, installApiDouble } from '../journeys/api-double';

// The sign-in page (SPEC-019 R-UX-LGN): the notice of a failed sign-in does not push the button out of the window,
// a phone shows only the form, and signing out lands here with a notice. They run against the production build and
// the stand-in API.

async function prepare(page: Page, size: { width: number; height: number }) {
  await installApiDouble(page);
  await page.addInitScript(() => {
    window.localStorage.setItem('orcon.locale', 'en');
  });
  await page.setViewportSize(size);
  await page.goto('/login');
  await expect(page.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
}

/** Fills the form with a wrong password and sends it: the page shows the failed sign-in notice. */
async function failToSignIn(page: Page) {
  await page.getByLabel(/^Email/).fill(account.email);
  await page.getByLabel(/^Password/).fill('not-the-password-1234');
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('alert')).toContainText('Email or password is incorrect.');
}

for (const size of [
  { width: 390, height: 844 },
  { width: 1440, height: 900 },
]) {
  test(`sign-in error keeps the submit button in view at ${String(size.width)}x${String(size.height)}`, async ({ page }) => {
    await prepare(page, size);
    await failToSignIn(page);
    const button = page.getByRole('button', { name: 'Sign in' });
    // The button is inside the window, whole, with the notice showing above the fields.
    await expect(button).toBeInViewport({ ratio: 1 });
    const alert = await page.getByRole('alert').boundingBox();
    const fields = await page.getByLabel(/^Email/).boundingBox();
    expect(alert?.y ?? Number.POSITIVE_INFINITY).toBeLessThan(fields?.y ?? 0);
  });
}

test('phone sign-in shows only the form', async ({ page }) => {
  await prepare(page, { width: 390, height: 844 });
  // The brand panel (headline, reasons, mascot) is not drawn; the form and the logo are.
  await expect(page.getByRole('heading', { level: 2, name: /Build, orchestrate and execute/ })).toBeHidden();
  await expect(page.getByText('Open source (Apache-2.0). Your data, on your server.')).toBeHidden();
  await expect(page.getByRole('img', { name: 'Orcon' }).first()).toBeVisible();
  await expect(page.getByLabel(/^Email/)).toBeVisible();
  await expect(page.getByLabel(/^Password/)).toBeVisible();
  // Language and theme are at the top right of the page.
  await expect(page.getByRole('button', { name: 'English' })).toBeVisible();
  await expect(page.getByRole('button', { name: /Switch to the (dark|light) theme/ })).toBeVisible();
});

test('the brand panel shows its reasons on a computer', async ({ page }) => {
  await prepare(page, { width: 1440, height: 900 });
  await expect(page.getByRole('heading', { level: 2, name: /Build, orchestrate and execute/ })).toBeVisible();
  await expect(page.getByText('Open source (Apache-2.0). Your data, on your server.')).toBeVisible();
});

test('the brand panel keeps its dark look in the light theme', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('orcon.theme', 'light');
  });
  await prepare(page, { width: 1440, height: 900 });
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  // The page is light and the panel is the surface color of the dark theme, resolved by the browser itself.
  const darkSurface = await page.evaluate(() => {
    const probe = document.createElement('div');
    probe.setAttribute('data-theme', 'dark');
    probe.style.backgroundColor = 'var(--orcon-surface-default)';
    document.body.append(probe);
    const color = getComputedStyle(probe).backgroundColor;
    probe.remove();
    return color;
  });
  expect(await page.locator('main').evaluate((element) => getComputedStyle(element).backgroundColor)).not.toBe(darkSurface);
  expect(await page.locator('aside').evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(darkSurface);
});

test('sign out lands on login with a notice', async ({ page }) => {
  await installApiDouble(page, { signedIn: true });
  await page.addInitScript(() => {
    window.localStorage.setItem('orcon.locale', 'en');
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/orgs/acme/overview');
  await expect(page.getByRole('heading', { level: 1, name: /^Good (morning|afternoon|evening)/ })).toBeVisible();
  await page.locator('button[aria-haspopup="menu"]:visible').last().click();
  await page.getByRole('menuitem', { name: 'Sign out' }).click();
  await expect(page).toHaveURL(/\/login\?signedOut=true$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
  await expect(page.getByText('You signed out.')).toBeVisible();
});
