// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

import { account, installApiDouble, org } from './api-double';

// The sign-in journey (SPEC-016 §3.8, SH8): sign in, create a project, create a flow, open it in the
// editor, reload, sign out, and come back to the address asked for after signing in. Every page is checked
// for accessibility violations (WCAG 2.2 AA). Without a server the journey runs against the stand-in API;
// with ORCON_JOURNEY_BASE_URL set it runs against a real server, using ORCON_JOURNEY_EMAIL and
// ORCON_JOURNEY_PASSWORD for the account of the first run, and ORCON_JOURNEY_ORG for its organization.

const live = process.env['ORCON_JOURNEY_BASE_URL'] !== undefined;
const liveEmail = process.env['ORCON_JOURNEY_EMAIL'] ?? '';
const liveSecret = process.env['ORCON_JOURNEY_PASSWORD'] ?? '';
const orgSlug = live ? (process.env['ORCON_JOURNEY_ORG'] ?? org.slug) : org.slug;
const email = live ? liveEmail : account.email;
const secret = live ? liveSecret : account.password;

const wcag = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

async function expectNoAccessibilityViolations(page: Page) {
  const results = await new AxeBuilder({ page }).withTags(wcag).analyze();
  const report = results.violations.map((violation) => `${violation.id}: ${violation.help}`);
  expect(report).toEqual([]);
}

async function signIn(page: Page, password: string) {
  await page.getByLabel(/^Email/).fill(email);
  await page.getByLabel(/^Password/).fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
}

test.skip(live && (liveEmail === '' || liveSecret === ''), 'The live journey needs ORCON_JOURNEY_EMAIL and ORCON_JOURNEY_PASSWORD.');

test.beforeEach(async ({ page }) => {
  if (!live) await installApiDouble(page);
  await page.addInitScript(() => {
    window.localStorage.setItem('orcon.locale', 'en');
  });
});

test('signing in leads to a project, its flow and the editor, and back', async ({ page }) => {
  const suffix = Date.now().toString(36);
  const projectName = `Sales ${suffix}`;
  const projectHandle = `sales-${suffix}`;

  // 1. A wrong password gets the same answer as an unknown account: nothing says which part was wrong.
  await page.goto('/login');
  await signIn(page, 'not-the-password-1234');
  await expect(page.getByText('Email or password is incorrect.')).toBeVisible();
  await expectNoAccessibilityViolations(page);

  // 2. The right password opens the home page of the organization.
  await signIn(page, secret);
  await expect(page).toHaveURL(new RegExp(`/orgs/${orgSlug}/overview$`));
  await expect(page.getByRole('heading', { level: 1, name: /^Good (morning|afternoon|evening)/ })).toBeVisible();
  await expectNoAccessibilityViolations(page);

  // 3. A new project, started from the sidebar, opens its flows.
  await page.getByRole('button', { name: 'Create a project' }).first().click();
  await page.getByLabel(/^Name/).fill(projectName);
  await page.getByRole('button', { name: 'Create project' }).click();
  await expect(page).toHaveURL(new RegExp(`/orgs/${orgSlug}/projects/${projectHandle}/flows$`));
  await expect(page.getByRole('heading', { level: 1, name: 'Flows' })).toBeVisible();

  // 4. A new flow opens in the editor at its own address.
  await page.getByRole('button', { name: 'Create flow' }).first().click();
  await page.getByLabel(/^Flow name/).fill('Refunds');
  await page.getByRole('button', { name: 'Create and open' }).click();
  await expect(page).toHaveURL(/\/flows\/[0-9a-f-]{36}$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Refunds' })).toBeVisible();
  const editorAddress = page.url();

  // 5. A reload keeps the same screen.
  await page.reload();
  await expect(page.getByRole('heading', { level: 1, name: 'Refunds' })).toBeVisible();
  expect(page.url()).toBe(editorAddress);
  await expectNoAccessibilityViolations(page);

  // 6. Signing out leaves the signed-in pages; the flows list is one address away.
  await page.goto(`/orgs/${orgSlug}/projects/${projectHandle}/flows`);
  await expect(page.getByRole('heading', { level: 1, name: 'Flows' })).toBeVisible();
  // The account button is the last menu button that is on screen (the sidebar's on a computer, the avatar of the top
  // bar on a phone); its name is the person's, whoever they are.
  await page.locator('button[aria-haspopup="menu"]:visible').last().click();
  await page.getByRole('menuitem', { name: 'Sign out' }).click();
  await expect(page).toHaveURL(/\/login\?signedOut=true$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
  await expect(page.getByText('You signed out.')).toBeVisible();

  // 7. A signed-out visit to a signed-in address asks to sign in, and returns there afterwards.
  const asked = `/orgs/${orgSlug}/projects/${projectHandle}/flows`;
  await page.goto(asked);
  await expect(page).toHaveURL(/\/login\?redirect=/);
  await signIn(page, secret);
  await expect(page).toHaveURL(new RegExp(`${asked}$`));
  await expect(page.getByRole('heading', { level: 1, name: 'Flows' })).toBeVisible();
  await expectNoAccessibilityViolations(page);
});
