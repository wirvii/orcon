// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

import { goInApp, mockDraftApi } from './draft-api';

// The node details view in a real browser, against the production build: the dialog and the docked
// panel are laid out by the browser, and the lists of the Select fields open in a layer above the
// dialog, which jsdom cannot show. The draft of the sample flow comes from a page-level API answer.

const FLOW = '/flows/01930000-0000-7000-8000-00000000ed5a';
const wcag = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

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

/** Opens the details of a node from the keyboard: focus the tile and press Enter. */
async function openDetails(page: Page, id: string, layout: 'dialog' | 'docked') {
  await node(page, id).focus();
  await page.keyboard.press('Enter');
  await page.waitForSelector(`[data-node-details="${layout}"]`);
}

async function expectNoViolations(page: Page) {
  const results = await new AxeBuilder({ page }).withTags(wcag).analyze();
  const report = results.violations.map((violation) => `${violation.id}: ${violation.help} (${violation.nodes.map((entry) => entry.target.join(' ')).join(', ')})`);
  expect(report).toEqual([]);
}

const layouts = [
  { name: 'desktop', width: 1440, height: 900, layout: 'dialog' },
  { name: 'tablet', width: 820, height: 1180, layout: 'docked' },
] as const;

for (const theme of ['light', 'dark'] as const) {
  for (const { name, width, height, layout } of layouts) {
    test(`the details view has no accessibility violations in the ${theme} theme on a ${name}, in every tab`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await openEditor(page, { theme });
      await openDetails(page, 'slack', layout);
      await expect(page.getByRole('tab', { name: 'Parameters' })).toHaveAttribute('aria-selected', 'true');
      await expectNoViolations(page);
      await page.getByRole('tab', { name: 'Settings' }).click();
      await expect(page.getByRole('textbox', { name: /Attempts in total/ })).toBeVisible();
      await expectNoViolations(page);
      await page.getByRole('tab', { name: 'Docs' }).click();
      await expect(page.getByRole('heading', { name: 'Operations' })).toBeVisible();
      await expectNoViolations(page);
    });
  }
}

test('the details view has no accessibility violations in compact density, with the changelog open', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await openEditor(page, { theme: 'dark', density: 'compact' });
  await openDetails(page, 'slack', 'dialog');
  await page.getByRole('button', { name: 'What changed' }).click();
  await expect(page.getByRole('heading', { name: 'Version 1.2.0' })).toBeVisible();
  await expectNoViolations(page);
});

test('the create credential dialog and an open list have no accessibility violations', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await openEditor(page, { theme: 'dark' });
  await openDetails(page, 'slack', 'dialog');
  await page.getByRole('combobox', { name: /Method/ }).click();
  // The list opens above the dialog and not behind it.
  const option = page.getByRole('option', { name: 'PUT' });
  await expect(option).toBeVisible();
  await option.click();
  await expect(page.getByRole('combobox', { name: /Method/ })).toHaveText('PUT');
  await page.getByRole('button', { name: 'Create new credential' }).click();
  const dialog = page.getByRole('dialog', { name: 'Create credential' });
  await expect(dialog).toBeVisible();
  await expectNoViolations(page);
  await dialog.getByRole('button', { name: 'Create credential' }).click();
  await expect(dialog.getByText('This field is required.').first()).toBeVisible();
  await expectNoViolations(page);
});

for (const locale of ['en', 'es'] as const) {
  test(`at exactly 1024 px the three panes fit inside the dialog and the data headers stay on one line in ${locale}`, async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await page.addInitScript((value) => {
      window.localStorage.setItem('orcon.locale', value);
    }, locale);
    await mockDraftApi(page);
    await page.goto(FLOW);
    await page.waitForSelector('[data-fitted="true"]');
    await openDetails(page, 'slack', 'dialog');
    const measures = await page.evaluate(() => {
      const dialog = document.querySelector('[data-node-details="dialog"]');
      if (!dialog) throw new Error('no dialog');
      const right = (selector: string) => dialog.querySelector(selector)?.getBoundingClientRect().right ?? 0;
      return {
        dialog: dialog.getBoundingClientRect().right,
        output: right('[data-pane="output"]'),
        headers: Array.from(dialog.querySelectorAll('[data-data-pane] > header')).map((header) => Math.round(header.getBoundingClientRect().height)),
      };
    });
    expect(measures.output).toBeLessThanOrEqual(measures.dialog);
    // One line: the header of a data pane is a single row of controls.
    expect(new Set(measures.headers).size).toBe(1);
    expect(measures.headers[0]).toBeLessThan(50);
  });
}

test('a dialog opened from the details view dims it: the details are under the backdrop of the new dialog', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await openEditor(page);
  await openDetails(page, 'slack', 'dialog');
  await page.getByRole('button', { name: 'Create new credential' }).click();
  await expect(page.getByRole('dialog', { name: 'Create credential' })).toBeVisible();
  // A point of the details view outside the small dialog lands on the backdrop of the new one, not on the details.
  const covered = await page.evaluate(() => {
    const top = document.elementFromPoint(100, 450);
    return top?.closest('[data-node-details]') === null;
  });
  expect(covered).toBe(true);
});

test.describe('on a computer', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('opens with Enter, keeps focus inside, and closes with Escape back to the node', async ({ page }) => {
    await openEditor(page);
    await openDetails(page, 'slack', 'dialog');
    const dialog = page.getByRole('dialog', { name: 'Slack details' });
    await expect(dialog.locator('[data-pane="input"]')).toBeVisible();
    await expect(dialog.locator('[data-pane="parameters"]')).toBeVisible();
    await expect(dialog.locator('[data-pane="output"]')).toBeVisible();
    // Tabbing never leaves the dialog.
    for (let step = 0; step < 40; step += 1) {
      await page.keyboard.press('Tab');
      expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true);
    }
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expect(node(page, 'slack')).toBeFocused();
  });

  test('opens with a double click and closes with Back to canvas, and focus returns to the node', async ({ page }) => {
    await openEditor(page);
    await node(page, 'inventory').dblclick();
    const dialog = page.getByRole('dialog', { name: 'HTTP Request details' });
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: 'Back to canvas' }).click();
    await expect(dialog).toHaveCount(0);
    await expect(node(page, 'inventory')).toBeFocused();
  });

  test('a trigger has no input pane', async ({ page }) => {
    await openEditor(page);
    await openDetails(page, 'webhook', 'dialog');
    const dialog = page.getByRole('dialog', { name: 'Webhook details' });
    await expect(dialog.locator('[data-pane="input"]')).toHaveCount(0);
    await expect(dialog.getByRole('textbox', { name: /Path/ })).toHaveValue('/orders');
  });

  test('the canvas shows what the form changes, and Undo takes it back', async ({ page }) => {
    await openEditor(page);
    await openDetails(page, 'slack', 'dialog');
    const dialog = page.getByRole('dialog', { name: 'Slack details' });
    await dialog.getByRole('textbox', { name: /URL/ }).fill('https://other.example.test/hook');
    await dialog.getByRole('tab', { name: 'Settings' }).click();
    await dialog.getByRole('combobox', { name: /After the last attempt fails/ }).click();
    await page.getByRole('option', { name: /error output/ }).click();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole('group', { name: /^Slack, POST other\.example\.test/ })).toBeVisible();
    await expect(node(page, 'slack').getByText('error', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Undo', exact: true })).toBeEnabled();
    await node(page, 'slack').focus();
    await page.keyboard.press('ControlOrMeta+z');
    await expect(node(page, 'slack').getByText('error', { exact: true })).toHaveCount(0);
    await page.keyboard.press('ControlOrMeta+z');
    await expect(page.getByRole('group', { name: /^Slack, POST hooks\.slack\.example\.test/ })).toBeVisible();
  });

  test('updates the connector version with one click, and Undo goes back to the old one', async ({ page }) => {
    await openEditor(page);
    await openDetails(page, 'slack', 'dialog');
    const dialog = page.getByRole('dialog', { name: 'Slack details' });
    await expect(dialog.getByText('New version available')).toBeVisible();
    await dialog.getByRole('button', { name: 'Update this node' }).click();
    await expect(dialog.getByText('New version available')).toHaveCount(0);
    // Version 1.2.0 brings the options the pinned version did not have.
    await expect(dialog.getByRole('switch', { name: 'Follow redirects to other hosts' })).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await node(page, 'slack').focus();
    await page.keyboard.press('ControlOrMeta+z');
    await openDetails(page, 'slack', 'dialog');
    await expect(page.getByRole('dialog', { name: 'Slack details' }).getByText('New version available')).toBeVisible();
  });

  test('creates a credential from the node and chooses it', async ({ page }) => {
    await openEditor(page);
    await openDetails(page, 'inventory', 'dialog');
    const dialog = page.getByRole('dialog', { name: 'HTTP Request details' });
    await dialog.getByRole('button', { name: 'Create new credential' }).click();
    const form = page.getByRole('dialog', { name: 'Create credential' });
    await form.getByRole('textbox', { name: /^Name/ }).fill('Warehouse key');
    await form.getByLabel(/Header value/).fill('s3cret');
    await form.getByRole('button', { name: 'Create credential' }).click();
    await expect(form).toHaveCount(0);
    await expect(dialog.getByRole('combobox', { name: /Authentication/ })).toContainText('Warehouse key');
    // The secret is nowhere on the page after it was sent.
    expect(await page.evaluate(() => document.body.innerHTML.includes('s3cret'))).toBe(false);
  });

  test('asks nothing from the network while the details are edited and a credential is created', async ({ page }) => {
    const requests: string[] = [];
    page.on('request', (request) => {
      if (['fetch', 'xhr', 'websocket', 'eventsource'].includes(request.resourceType())) requests.push(request.url());
    });
    await openEditor(page);
    await openDetails(page, 'slack', 'dialog');
    const dialog = page.getByRole('dialog', { name: 'Slack details' });
    await dialog.getByRole('textbox', { name: /URL/ }).fill('https://other.example.test/hook');
    await dialog.getByRole('button', { name: 'Create new credential' }).click();
    const form = page.getByRole('dialog', { name: 'Create credential' });
    await form.getByRole('textbox', { name: /^Name/ }).fill('Another key');
    await form.getByLabel(/Header value/).fill('x');
    await form.getByRole('button', { name: 'Create credential' }).click();
    await expect(form).toHaveCount(0);
    // The only data the editor asks for is its draft; a credential is created in the page, without a request.
    expect(requests.filter((url) => !url.endsWith('/draft'))).toEqual([]);
  });

  test('says test runs come later when a pane or the header offers the run', async ({ page }) => {
    await openEditor(page);
    await openDetails(page, 'slack', 'dialog');
    const dialog = page.getByRole('dialog', { name: 'Slack details' });
    await dialog.locator('[data-pane="input"]').getByRole('button', { name: 'Execute previous nodes' }).click();
    await expect(page.getByText('Test runs arrive in a later editor release.')).toBeVisible();
  });

  test('keeps the sample flow in the document: the details edit survives leaving and coming back', async ({ page }) => {
    await openEditor(page);
    await openDetails(page, 'slack', 'dialog');
    await page.getByRole('dialog', { name: 'Slack details' }).getByRole('textbox', { name: /URL/ }).fill('https://kept.example.test/hook');
    await page.keyboard.press('Escape');
    await expect(page.getByRole('status').filter({ hasText: 'Saved' })).toBeVisible({ timeout: 6_000 });
    await page.getByRole('link', { name: 'Settings' }).first().click();
    await goInApp(page, FLOW);
    await page.waitForSelector('[data-fitted="true"]');
    await expect(page.getByRole('group', { name: /^Slack, POST kept\.example\.test/ })).toBeVisible();
  });
});

test.describe('on a tablet', () => {
  test.use({ viewport: { width: 820, height: 1180 } });

  test('docks the parameters beside the canvas and the data under it, without horizontal scrolling', async ({ page }) => {
    await openEditor(page);
    await openDetails(page, 'slack', 'docked');
    const docked = page.getByRole('complementary', { name: 'Slack details' });
    await expect(docked).toBeVisible();
    await expect(page.getByRole('region', { name: 'Data of Slack' })).toBeVisible();
    await expect(node(page, 'slack')).toBeVisible();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
    expect(overflow).toBe(false);
    const paneOverflow = await docked.evaluate((element) => element.scrollWidth > element.clientWidth);
    expect(paneOverflow).toBe(false);
  });

  test('closes with Escape from a field and gives focus back to the node', async ({ page }) => {
    await openEditor(page);
    await openDetails(page, 'slack', 'docked');
    await page.getByRole('complementary', { name: 'Slack details' }).getByRole('textbox', { name: /URL/ }).click();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('complementary', { name: 'Slack details' })).toHaveCount(0);
    await expect(node(page, 'slack')).toBeFocused();
  });

  test('a Call sub-flow node pasted into the flow has its widget inside the panel, and the link opens the called flow', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await openEditor(page);
    const fragment = {
      format: 'orcon/flow-fragment@1',
      nodes: [
        {
          id: 'ship',
          name: 'Ship order',
          element: 'callActivity',
          type: 'orcon.callFlow@1',
          parameters: {},
          call: { target: '01930000-0000-7000-8000-00000000ed61', version: 'CURRENT', mode: 'WAIT', perItem: false },
        },
      ],
      edges: [],
      editor: { nodes: { ship: { x: 0, y: 0 } } },
    };
    await page.evaluate((text) => navigator.clipboard.writeText(text), JSON.stringify(fragment));
    await node(page, 'done').click();
    await page.keyboard.press('ControlOrMeta+v');
    await expect(page.getByText('Pasted 1 item')).toBeVisible();
    await page.getByRole('group', { name: /^Ship order,/ }).focus();
    await page.keyboard.press('Enter');
    const docked = page.getByRole('complementary', { name: 'Ship order details' });
    await expect(docked).toBeVisible();
    await expect(docked.getByRole('combobox', { name: /Flow/ })).toContainText('Notify the team');
    const paneOverflow = await docked.evaluate((element) => element.scrollWidth > element.clientWidth);
    expect(paneOverflow).toBe(false);
    await expectNoViolations(page);
    await docked.getByRole('link', { name: /Open sub-flow/ }).click();
    await expect(page).toHaveURL(/00000000ed61$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Notify the team' })).toBeVisible();
  });
});

// On a phone the details view opens full screen and only shows: every value is there, and no control can change it.
test.describe('on a phone the node details are read-only', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  /** The controls of the open view that a person can still press or type into, except the tabs and the close button. */
  async function enabledControls(page: Page): Promise<string[]> {
    return page.locator('[data-node-details="fullscreen"]').evaluate((root) => {
      const labelOf = (element: HTMLElement) => element.getAttribute('aria-label') ?? element.textContent.trim();
      return Array.from(root.querySelectorAll<HTMLElement>('input, textarea, select, button, [role="combobox"], [role="switch"], [role="checkbox"], [contenteditable="true"]'))
        .filter((element) => !element.matches(':disabled') && element.getAttribute('aria-disabled') !== 'true')
        .filter((element) => element.getAttribute('role') !== 'tab' && labelOf(element) !== 'Back to canvas')
        .map((element) => `${element.tagName.toLowerCase()} ${labelOf(element)}`.trim());
    });
  }

  for (const theme of ['light', 'dark'] as const) {
    test(`the read-only details have no accessibility violations in the ${theme} theme, in every tab`, async ({ page }) => {
      await openEditor(page, { theme });
      await node(page, 'slack').click();
      await page.waitForSelector('[data-node-details="fullscreen"]');
      await expect(page.getByRole('tab', { name: 'Parameters' })).toHaveAttribute('aria-selected', 'true');
      await expectNoViolations(page);
      await page.getByRole('tab', { name: 'Settings' }).click();
      await expect(page.getByRole('textbox', { name: /Attempts in total/ })).toBeDisabled();
      await expectNoViolations(page);
      await page.getByRole('tab', { name: 'Docs' }).click();
      await expect(page.getByRole('heading', { name: 'Operations' })).toBeVisible();
      await expectNoViolations(page);
    });
  }

  test('a tap on a node opens the details with every value shown and no control that writes, and the close button gives focus back to the node', async ({ page }) => {
    await openEditor(page);
    await node(page, 'slack').click();
    const view = page.locator('[data-node-details="fullscreen"]');
    await expect(view).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Parameters' })).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByText('Read-only on this screen')).toBeVisible();
    // The short summary a phone used to show is not opened: the full view is.
    await expect(page.locator('[data-node-details="slack"]')).toHaveCount(0);
    await expect(view.getByRole('textbox').first()).toBeDisabled();
    expect(await enabledControls(page)).toEqual([]);
    await page.getByRole('tab', { name: 'Settings' }).click();
    await expect(page.getByRole('textbox', { name: /Attempts in total/ })).toBeDisabled();
    expect(await enabledControls(page)).toEqual([]);
    await page.getByRole('tab', { name: 'Docs' }).click();
    await expect(page.getByRole('heading', { name: 'Operations' })).toBeVisible();
    expect(await enabledControls(page)).toEqual([]);
    // The view fits the screen: nothing scrolls sideways.
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.getByRole('button', { name: 'Back to canvas' }).click();
    await expect(view).toBeHidden();
    await expect(node(page, 'slack')).toBeFocused();
  });
});

