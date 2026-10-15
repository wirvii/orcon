// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

import { goInApp, mockDraftApi } from './draft-api';

// The flow editor in a real browser, against the production build: the design system's canvas
// measures nodes and draws connections, which jsdom cannot do. The preview opens a sample flow kept
// in memory, so no request leaves the page.

const FLOW = '/flows/01930000-0000-7000-8000-00000000ed5a';
const wcag = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

const viewports = [
  { name: 'phone', width: 390, height: 844 },
  { name: 'tablet', width: 820, height: 1180 },
  { name: 'desktop', width: 1440, height: 900 },
] as const;

async function openEditor(page: Page, options: { theme?: 'light' | 'dark'; density?: 'comfortable' | 'compact' } = {}) {
  await mockDraftApi(page);
  await page.addInitScript(
    ({ theme, density }) => {
      window.localStorage.setItem('orcon.locale', 'en');
      window.localStorage.setItem('orcon.theme', theme);
      window.localStorage.setItem('orcon.density', density);
    },
    { theme: options.theme ?? 'light', density: options.density ?? 'comfortable' },
  );
  await page.goto(FLOW);
  await expect(page.getByRole('heading', { level: 1, name: 'Order notifications' })).toBeVisible();
  await page.waitForSelector('[data-fitted="true"]');
}

const node = (page: Page, id: string) => page.locator(`.react-flow__node[data-id="${id}"]`);
const tiles = (page: Page) => page.locator('.react-flow__node-tile');
const banner = async (page: Page) => (await page.locator('[role="status"]').allInnerTexts()).join(' | ').replace(/\s+/g, ' ');

/** Where a node is on the page now (its box), so a test can tell that the view did not move. */
async function boxOf(page: Page, id: string) {
  const box = await node(page, id).boundingBox();
  if (!box) throw new Error(`Node ${id} has no box.`);
  return { x: Math.round(box.x), y: Math.round(box.y) };
}

test('the editor opens at its stable address', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('orcon.locale', 'en');
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await mockDraftApi(page);
  await page.goto(FLOW);
  await expect(page).toHaveURL(new RegExp(`${FLOW}$`));
  await expect(page.getByRole('heading', { level: 1, name: 'Order notifications' })).toBeVisible();
  // A reload of that address (the server answers every address with the page) opens the same flow.
  await page.reload();
  await expect(page.getByRole('heading', { level: 1, name: 'Order notifications' })).toBeVisible();
});

test('the navigation links to the API reference, a page the server serves, in the same tab', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem('orcon.locale', 'en');
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/settings/appearance');
  const link = page.getByRole('link', { name: 'API reference' }).first();
  await expect(link).toHaveAttribute('href', '/api/reference/');
  await expect(link).not.toHaveAttribute('target', '_blank');
});

test('an address that is not a flow id is a page that does not exist', async ({ page }) => {
  await page.goto('/flows/not-a-flow');
  await expect(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
});

for (const theme of ['light', 'dark'] as const) {
  for (const viewport of viewports) {
    test(`the editor has no accessibility violations in the ${theme} theme on a ${viewport.name}`, async ({ page }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });
      await openEditor(page, { theme });
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
      const results = await new AxeBuilder({ page }).withTags(wcag).analyze();
      const report = results.violations.map((violation) => `${violation.id}: ${violation.help} (${violation.nodes.map((entry) => entry.target.join(' ')).join(', ')})`);
      expect(report).toEqual([]);
    });
  }
}

test('the dialogs and the outline have no accessibility violations either', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await openEditor(page, { theme: 'dark', density: 'compact' });
  await page.getByRole('button', { name: 'Flow outline', exact: true }).click();
  await expect(page.getByRole('complementary', { name: 'Flow outline' })).toBeVisible();
  let results = await new AxeBuilder({ page }).withTags(wcag).analyze();
  expect(results.violations.map((violation) => violation.id)).toEqual([]);
  await page.getByRole('button', { name: 'Keyboard shortcuts', exact: true }).click();
  await expect(page.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeVisible();
  results = await new AxeBuilder({ page }).withTags(wcag).analyze();
  expect(results.violations.map((violation) => violation.id)).toEqual([]);
});

test('the editor reads its draft from the API and asks for nothing else', async ({ page }) => {
  const requests: string[] = [];
  page.on('request', (request) => {
    // The files of the page are not requests of the editor; a data request (fetch, XHR, a socket or a stream) would be.
    if (['fetch', 'xhr', 'websocket', 'eventsource'].includes(request.resourceType())) requests.push(request.url());
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await openEditor(page);
  expect(requests.length).toBeGreaterThan(0);
  expect(requests.every((url) => url.endsWith(`/api/v1/flows/${FLOW.split('/').pop() ?? ''}/draft`))).toBe(true);
});

test.describe('on a phone the flow is read-only', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('shows the flow without any control that edits it', async ({ page }) => {
    await openEditor(page);
    await expect(page.getByText(/View only/)).toBeVisible();
    for (const name of ['Add node', 'Undo', 'Tidy up', 'Save now', 'Keyboard shortcuts']) {
      await expect(page.getByRole('button', { name, exact: true })).toHaveCount(0);
    }
    await expect(page.locator('[data-workflow-canvas]')).toHaveAttribute('data-editable', 'false');
    await expect(tiles(page)).toHaveCount(6);
  });
});

test.describe('on a computer', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('undoes and redoes a move without moving the view, with the keyboard', async ({ page }) => {
    await openEditor(page);
    const before = await boxOf(page, 'slack');
    await node(page, 'slack').focus();
    // Two grid steps to the right, then a coarse step down: the keyboard path of a drag (R-ED-CAN-02).
    await page.keyboard.press('Shift+ArrowRight');
    await page.keyboard.press('Shift+ArrowRight');
    await page.keyboard.press('Alt+Shift+ArrowDown');
    const moved = await boxOf(page, 'slack');
    expect(moved.x).toBeGreaterThan(before.x);
    expect(moved.y).toBeGreaterThan(before.y);
    await expect(page.getByRole('button', { name: 'Undo', exact: true })).toBeEnabled();

    await page.keyboard.press('ControlOrMeta+z');
    await page.keyboard.press('ControlOrMeta+z');
    await page.keyboard.press('ControlOrMeta+z');
    await expect(page.getByRole('button', { name: 'Undo', exact: true })).toBeDisabled();
    expect(await boxOf(page, 'slack')).toEqual(before);

    await page.keyboard.press('ControlOrMeta+Shift+z');
    await expect(page.getByRole('button', { name: 'Redo', exact: true })).toBeEnabled();
    await page.keyboard.press('ControlOrMeta+y');
    await page.keyboard.press('ControlOrMeta+y');
    await expect(page.getByRole('button', { name: 'Redo', exact: true })).toBeDisabled();
    expect(await boxOf(page, 'slack')).toEqual(moved);
  });

  test('adds a node from the palette, names it so the name is free, and the toolbar undoes it', async ({ page }) => {
    await openEditor(page);
    await page.getByRole('button', { name: 'Add node', exact: true }).click();
    await page.getByRole('option', { name: /^HTTP Request/ }).click();
    await expect(tiles(page)).toHaveCount(7);
    await expect(page.getByRole('group', { name: /^HTTP Request 2,/ })).toBeVisible();
    // One action, one step: the name the canvas takes afterwards is not a second step.
    await expect(node(page, 'add-http-1')).toBeFocused();
    await page.keyboard.press('ControlOrMeta+z');
    await expect(tiles(page)).toHaveCount(6);
    await expect(page.getByRole('button', { name: 'Undo', exact: true })).toBeDisabled();
    await page.getByRole('button', { name: 'Redo', exact: true }).click();
    await expect(tiles(page)).toHaveCount(7);
    await page.getByRole('button', { name: 'Undo', exact: true }).click();
    await expect(tiles(page)).toHaveCount(6);
    await page.getByRole('button', { name: 'Redo', exact: true }).click();
    await expect(tiles(page)).toHaveCount(7);
  });

  test('renames a node and says how many references it rewrote, then undoes it', async ({ page }) => {
    await openEditor(page);
    await node(page, 'webhook').click();
    await page.keyboard.press('F2');
    await page.keyboard.type('Order received');
    await page.keyboard.press('Enter');
    await expect(page.getByText('Renamed to Order received. 1 reference updated')).toBeVisible();
    await page.keyboard.press('ControlOrMeta+z');
    await expect(page.getByRole('group', { name: /^Webhook,/ })).toBeVisible();
  });

  test('shows the undo notice after deleting nodes, and its Undo brings them back in step with the store', async ({ page }) => {
    await openEditor(page);
    await node(page, 'slack').focus();
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.press('Delete');
    await page.getByRole('alertdialog').getByRole('button', { name: 'Delete nodes' }).click();
    await expect(tiles(page)).toHaveCount(0);
    const toast = page.locator('[data-sonner-toast]');
    await expect(toast).toContainText(/Deleted \d+ nodes/);
    await toast.getByRole('button', { name: 'Undo' }).click();
    await expect(tiles(page)).toHaveCount(6);
    // The store followed: the flow is saved with its nodes, and history still goes back through the delete.
    await expect(page.getByRole('status').filter({ hasText: 'Saved' })).toBeVisible({ timeout: 6_000 });
    await page.getByRole('link', { name: 'Settings' }).first().click();
    await goInApp(page, FLOW);
    await page.waitForSelector('[data-fitted="true"]');
    await expect(tiles(page)).toHaveCount(6);
  });

  test('tidies up the flow with the keyboard: the connections of the If do not cross, and it is one undo', async ({ page }) => {
    await openEditor(page);
    await node(page, 'rejected').focus();
    for (let step = 0; step < 4; step += 1) await page.keyboard.press('Alt+Shift+ArrowUp');
    await node(page, 'slack').focus();
    for (let step = 0; step < 3; step += 1) await page.keyboard.press('Alt+Shift+ArrowDown');
    await page.keyboard.press('Alt+Shift+t');
    await expect(page.getByText(/Tidied up \d+ items/)).toBeVisible();
    const yes = await boxOf(page, 'inventory');
    const no = await boxOf(page, 'rejected');
    expect(yes.y).toBeLessThan(no.y);
    expect(yes.x).toBe(no.x);
    await page.keyboard.press('ControlOrMeta+z');
    await expect(page.getByRole('button', { name: 'Redo', exact: true })).toBeEnabled();
    expect((await boxOf(page, 'rejected')).y).not.toBe(no.y);
  });

  test('copies two nodes and pastes them with new names, and duplicates with the keyboard', async ({ page }) => {
    await openEditor(page);
    await node(page, 'inventory').click();
    await node(page, 'slack').click({ modifiers: ['Shift'] });
    await page.keyboard.press('ControlOrMeta+c');
    await expect(page.getByText('Copied 2 items')).toBeVisible();
    await page.mouse.move(700, 700);
    await page.keyboard.press('ControlOrMeta+v');
    await expect(page.getByText('Pasted 2 items')).toBeVisible();
    await expect(tiles(page)).toHaveCount(8);
    await expect(page.getByRole('group', { name: /^HTTP Request 2,/ })).toBeVisible();
    await expect(page.getByRole('group', { name: /^Slack 2,/ })).toBeVisible();

    await page.keyboard.press('ControlOrMeta+z');
    await expect(tiles(page)).toHaveCount(6);
    await node(page, 'done').click();
    await page.keyboard.press('ControlOrMeta+d');
    await expect(tiles(page)).toHaveCount(7);
  });

  test('puts an Orcon fragment, and nothing else, on the clipboard', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await openEditor(page);
    await node(page, 'validate').click();
    await page.keyboard.press('ControlOrMeta+c');
    await expect(page.getByText('Copied 1 item')).toBeVisible();
    const text = await page.evaluate(() => navigator.clipboard.readText());
    const fragment: unknown = JSON.parse(text);
    expect(fragment).toMatchObject({ format: 'orcon/flow-fragment@1', nodes: [{ id: 'validate', type: 'orcon.if@1' }], edges: [] });
  });

  test('O moves focus into the outline, and Escape gives it back to the node', async ({ page }) => {
    await openEditor(page);
    await node(page, 'slack').focus();
    await page.keyboard.press('o');
    await expect(page.getByRole('complementary', { name: 'Flow outline' })).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('complementary', { name: 'Flow outline' })).toHaveCount(0);
    await expect(node(page, 'slack')).toBeFocused();
  });

  test('the toolbar is one tab stop and the arrow keys move between its buttons', async ({ page }) => {
    await openEditor(page);
    const toolbar = page.getByRole('toolbar', { name: 'Flow editor tools' });
    // Undo and Redo are disabled, so the stops are the buttons that can be used.
    expect(await toolbar.locator('button[tabindex="0"]').count()).toBe(1);
    await toolbar.getByRole('button', { name: 'Add node', exact: true }).focus();
    await page.keyboard.press('ArrowRight');
    await expect(toolbar.getByRole('button', { name: 'Add sticky note', exact: true })).toBeFocused();
    await page.keyboard.press('ArrowRight');
    // Undo and Redo are disabled and skipped.
    await expect(toolbar.getByRole('button', { name: 'Tidy up', exact: true })).toBeFocused();
    await page.keyboard.press('End');
    await expect(toolbar.getByRole('button', { name: 'Keyboard shortcuts', exact: true })).toBeFocused();
    await page.keyboard.press('ArrowRight');
    await expect(toolbar.getByRole('button', { name: 'Add node', exact: true })).toBeFocused();
    await page.keyboard.press('ArrowLeft');
    await expect(toolbar.getByRole('button', { name: 'Keyboard shortcuts', exact: true })).toBeFocused();
    // The button that had focus last is the tab stop.
    expect(await toolbar.locator('button[tabindex="0"]').count()).toBe(1);
    await expect(toolbar.locator('button[tabindex="0"]')).toHaveAccessibleName('Keyboard shortcuts');
    await page.keyboard.press('Tab');
    expect(await toolbar.evaluate((element) => element.contains(document.activeElement))).toBe(false);
  });

  test('finds a node by name, lists the shortcuts and refuses what comes in a later release', async ({ page }) => {
    await openEditor(page);
    await node(page, 'webhook').focus();
    await page.keyboard.press('ControlOrMeta+f');
    await page.getByRole('textbox', { name: 'Node name' }).fill('http');
    await page.keyboard.press('Enter');
    await expect(node(page, 'inventory')).toBeFocused();

    await page.keyboard.press('?');
    await expect(page.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeVisible();
    await page.keyboard.press('Escape');
    // Closing the dialog gives focus back, so the next shortcut still works.
    await expect(node(page, 'inventory')).toBeFocused();

    await page.keyboard.press('p');
    await expect(page.getByText('Pinning data arrives in a later editor release.')).toBeVisible();
    await page.keyboard.press('ControlOrMeta+Enter');
    await expect(page.getByText('Test runs arrive in a later editor release.')).toBeVisible();
  });

  test('saves the draft by itself and on request, and keeps it for the next visit within the session', async ({ page }) => {
    await openEditor(page);
    await node(page, 'slack').focus();
    await page.keyboard.press('d');
    await expect(page.getByRole('status').filter({ hasText: 'Unsaved changes' })).toBeVisible();
    await expect(page.getByRole('status').filter({ hasText: 'Saved' })).toBeVisible({ timeout: 6_000 });
    // Leave for another page and come back: the saved draft is what opens.
    await page.getByRole('link', { name: 'Settings' }).first().click();
    await expect(page.getByRole('heading', { level: 1, name: 'Appearance and language' })).toBeVisible();
    await goInApp(page, FLOW);
    await page.waitForSelector('[data-fitted="true"]');
    await expect(page.getByRole('group', { name: /^Slack,.*disabled/ })).toBeVisible();
    expect(await banner(page)).not.toContain('Unsaved');
  });
});
