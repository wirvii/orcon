// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

import { account, installApiDouble, org } from './api-double';

// The way from the home page to the editor and back (SPEC-019 UX-O2): sign in, reach Home, open a project, create
// a blank flow from the chooser, arrive in the editor and return. It runs on a computer and on a phone (the two
// projects of the journeys config). Without a server it runs against the stand-in API; with ORCON_JOURNEY_BASE_URL
// set it runs against a real server, using ORCON_JOURNEY_EMAIL, ORCON_JOURNEY_PASSWORD and ORCON_JOURNEY_ORG.

const live = process.env['ORCON_JOURNEY_BASE_URL'] !== undefined;
const liveEmail = process.env['ORCON_JOURNEY_EMAIL'] ?? '';
const liveSecret = process.env['ORCON_JOURNEY_PASSWORD'] ?? '';
const orgSlug = live ? (process.env['ORCON_JOURNEY_ORG'] ?? org.slug) : org.slug;
const email = live ? liveEmail : account.email;
const secret = live ? liveSecret : account.password;

const wcag = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

async function expectNoAccessibilityViolations(page: Page) {
  const results = await new AxeBuilder({ page }).withTags(wcag).analyze();
  expect(results.violations.map((violation) => `${violation.id}: ${violation.help}`)).toEqual([]);
}

test.skip(live && (liveEmail === '' || liveSecret === ''), 'The live journey needs ORCON_JOURNEY_EMAIL and ORCON_JOURNEY_PASSWORD.');

test.beforeEach(async ({ page }) => {
  if (!live) await installApiDouble(page);
  await page.addInitScript(() => {
    window.localStorage.setItem('orcon.locale', 'en');
  });
});

test('Home leads to a project, a blank flow, the editor and back', async ({ page }) => {
  const suffix = Date.now().toString(36);
  const projectName = `Journey ${suffix}`;
  const flowName = `Welcome ${suffix}`;

  // 1. Sign in: the first page is Home.
  await page.goto('/login');
  await page.getByLabel(/^Email/).fill(email);
  await page.getByLabel(/^Password/).fill(secret);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page).toHaveURL(new RegExp(`/orgs/${orgSlug}/overview$`));
  await expect(page.getByRole('heading', { level: 1, name: /^Good (morning|afternoon|evening)/ })).toBeVisible();

  // 2. A project, started from Home, opens its page with its own tabs.
  await page.getByRole('button', { name: 'Create a project' }).first().click();
  await page.getByLabel(/^Name/).fill(projectName);
  await page.getByRole('button', { name: 'Create project' }).click();
  await expect(page.getByRole('heading', { level: 1, name: projectName })).toBeVisible();
  const sections = page.getByRole('navigation', { name: 'Sections of the project' });
  await expect(sections.getByRole('link', { name: /^Flows/ })).toHaveAttribute('aria-current', 'page');
  // A project with no flow offers the first step.
  await expect(page.getByRole('heading', { name: `${projectName} is ready for its first flow` })).toBeVisible();
  await expectNoAccessibilityViolations(page);

  // 3. The chooser offers four ways to start; Blank creates an inactive flow and opens the editor.
  await page.getByRole('button', { name: 'Create flow' }).first().click();
  const dialog = page.getByRole('dialog', { name: 'Create flow' });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('group', { name: 'How to start' }).getByRole('button')).toHaveCount(4);
  await expectNoAccessibilityViolations(page);
  await dialog.getByLabel(/^Flow name/).fill(flowName);
  await dialog.getByRole('button', { name: 'Create and open' }).click();
  await expect(page).toHaveURL(/\/flows\/[0-9a-f-]{36}$/);
  await expect(page.getByRole('heading', { level: 1, name: flowName })).toBeVisible();

  // 4. Back to the project: the new flow is in the list, off and waiting for its first publication.
  await page.goBack();
  await expect(page.getByRole('heading', { level: 1, name: projectName })).toBeVisible();
  const row = page.getByRole('listitem').filter({ has: page.getByRole('link', { name: flowName }) });
  await expect(row).toBeVisible();
  await expect(row.getByRole('switch')).not.toBeChecked();
  await expect(row.getByRole('switch')).toHaveAttribute('aria-disabled', 'true');
  await expectNoAccessibilityViolations(page);

  // 5. Home lists the same flow with its project.
  await page.goto(`/orgs/${orgSlug}/overview`);
  const homeRow = page.getByRole('listitem').filter({ has: page.getByRole('link', { name: flowName }) });
  await expect(homeRow).toBeVisible();
  await expect(homeRow).toContainText(projectName);
  await expectNoAccessibilityViolations(page);
});
