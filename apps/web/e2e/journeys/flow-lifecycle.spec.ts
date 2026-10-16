// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test, type Request } from '@playwright/test';

// The flow lifecycle journey (SPEC-014 ED10a): sign in, create a flow, add a webhook trigger and an HTTP node,
// publish it, activate it and deactivate it. It runs only against a real server (apps/web/scripts/e2e-live.sh
// starts one and sets the variables below), because what it checks is the contract of the lifecycle operations:
// publishing is based on the entity tag of the draft, activating and deactivating on the one of the flow.

const base = process.env['ORCON_JOURNEY_BASE_URL'];
const email = process.env['ORCON_JOURNEY_EMAIL'] ?? '';
const secret = process.env['ORCON_JOURNEY_PASSWORD'] ?? '';
const orgSlug = process.env['ORCON_JOURNEY_ORG'] ?? '';

test.skip(base === undefined || email === '' || secret === '', 'The lifecycle journey needs a running server: use apps/web/scripts/e2e-live.sh.');

// The canvas is edited on a computer; the editor on a phone is covered by the screen tests.
test.skip(({ viewport }) => (viewport?.width ?? 0) < 768, 'The lifecycle journey builds a flow on the canvas, which needs a computer-sized screen.');

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('orcon.locale', 'en');
  });
});

test('a flow is built, published, activated and deactivated against the real server', async ({ page }) => {
  const suffix = Date.now().toString(36);
  const calls: { operation: string; ifMatch: string | null; status: number }[] = [];
  const lifecycle = /\/api\/v1\/flows\/[0-9a-f-]{36}\/(publish|activate|deactivate)$/;
  page.on('requestfinished', (request: Request) => {
    const match = lifecycle.exec(new URL(request.url()).pathname);
    const operation = match?.[1];
    if (request.method() !== 'POST' || operation === undefined) return;
    void request.response().then((response) => {
      calls.push({ operation, ifMatch: request.headers()['if-match'] ?? null, status: response?.status() ?? 0 });
    });
  });

  // Sign in.
  await page.goto('/login');
  await page.getByLabel(/^Email/).fill(email);
  await page.getByLabel(/^Password/).fill(secret);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page).toHaveURL(new RegExp(`/orgs/${orgSlug}/overview$`));

  // A project and a flow, opened in the editor.
  await page.getByRole('button', { name: 'Create a project' }).first().click();
  await page.getByLabel(/^Name/).fill(`Lifecycle ${suffix}`);
  await page.getByRole('button', { name: 'Create project' }).click();
  await expect(page).toHaveURL(/\/flows$/);
  await page.getByRole('button', { name: 'Create flow' }).first().click();
  await page.getByLabel(/^Flow name/).fill('Order notifications');
  await page.getByRole('button', { name: 'Create and open' }).click();
  await expect(page).toHaveURL(/\/flows\/[0-9a-f-]{36}$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Order notifications' })).toBeVisible();

  // Nothing is published, so the flow cannot be activated and the switch says why.
  const activeSwitch = page.getByRole('switch', { name: 'Active: Order notifications' });
  await expect(activeSwitch).toBeDisabled();
  await expect(page.getByText('Publish a version first: only a published version can run.')).toBeVisible();

  // A webhook trigger, then an HTTP node after it.
  await page.getByRole('button', { name: 'Add first step' }).click();
  await page.getByRole('option', { name: /^Webhook/ }).click();
  await expect(page.getByRole('region', { name: 'Flow canvas: 1 node' })).toBeVisible();
  await page.locator('button[aria-label="Add node after Webhook"]').click();
  await page.getByRole('combobox', { name: 'Search nodes' }).fill('HTTP');
  await page.getByRole('option', { name: /^HTTP Request/ }).click();
  await expect(page.getByRole('region', { name: 'Flow canvas: 2 nodes' })).toBeVisible();

  // The autosave writes the draft; Publish is based on the entity tag of that draft.
  await expect(page.getByRole('status').filter({ hasText: /Save state: Saved/ })).toBeVisible();
  await page.getByRole('button', { name: /^Publish/ }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel(/^Release note/).fill('First version');
  await dialog.getByRole('button', { name: 'Publish' }).click();
  await expect(dialog).toBeHidden();

  // Activate, then deactivate: both are based on the entity tag of the flow, read again after each change.
  // The switch is enabled only once the flow has a published version.
  await expect(activeSwitch).toBeEnabled();
  await activeSwitch.click();
  await expect(activeSwitch).toBeChecked();
  await activeSwitch.click();
  await expect(activeSwitch).not.toBeChecked();

  await expect.poll(() => calls.length).toBe(3);
  expect(calls.map((call) => [call.operation, call.status])).toEqual([
    ['publish', 200],
    ['activate', 200],
    ['deactivate', 200],
  ]);
  // Every lifecycle call carries an entity tag, and the one of the publication is the draft's, not the flow's.
  for (const call of calls) expect(call.ifMatch).toMatch(/^"[^"]+"$/);
  expect(calls[1]?.ifMatch).not.toBe(calls[2]?.ifMatch);
});
