// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { mkdirSync } from 'node:fs';
import { join } from 'node:path';

import { expect, test, type Page } from '@playwright/test';

import { mockDraftApi } from '../../test/draft-api';
import { account, installApiDouble, org } from '../journeys/api-double';

import { mockExecutions } from './executions-double';

const EDITOR_FLOW = '/orgs/acme/projects/ventas/flows/01930000-0000-7000-8000-00000000ed5a';

// Visual evidence for a pull request (SPEC-019 §8): the screens of the product at 1440x900 and 390x844, in the
// dark and light themes, next to the same screens of the approved mockup. It is not a pass/fail check, so it
// runs only through its own config (`playwright.ux-review.config.ts`) and never in `make check`.
//
//   ORCON_CAPTURE_LABEL=before|after   folder name under the output directory (default "after")
//   ORCON_CAPTURE_OUT                  output directory (default ".playwright-mcp/ux-progress" at the repository root)
//   ORCON_MOCKUP_URL                   address of the mockup index.html; when set, the mockup is captured instead
//   ORCON_CAPTURE_ONLY                 comma-separated screen names to capture

const label = process.env['ORCON_CAPTURE_LABEL'] ?? 'after';
const out = process.env['ORCON_CAPTURE_OUT'] ?? join(process.cwd(), '..', '..', '.playwright-mcp', 'ux-progress');
const mockupUrl = process.env['ORCON_MOCKUP_URL'];
const only = process.env['ORCON_CAPTURE_ONLY']?.split(',').filter(Boolean);

const themes = ['dark', 'light'] as const;
const devices = [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'phone', width: 390, height: 844 },
] as const;

type Device = (typeof devices)[number]['name'];

/** The screens of the mockup (its own tab id and variant) and how the product reaches the same state. */
interface Screen {
  name: string;
  mockup: { id: string; variant?: string };
  /** Skipped on a device when the screen does not exist there. */
  devices?: readonly Device[];
  product: (page: Page, device: Device) => Promise<void>;
}

async function seed(page: Page) {
  // Three projects and a few flows, created through the same API the screens use.
  await page.evaluate(
    async ({ orgId }) => {
      const create = async (path: string, body: unknown) => {
        const response = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
        return (await response.json()) as { id: string };
      };
      const ventas = await create(`/api/v1/orgs/${orgId}/projects`, { name: 'Ventas WhatsApp' });
      await create(`/api/v1/orgs/${orgId}/projects`, { name: 'Soporte' });
      await create(`/api/v1/orgs/${orgId}/projects`, { name: 'Operaciones' });
      for (const name of ['Agente de ventas', 'Recordatorio de pago', 'Calificar leads de Meta Ads', 'Confirmación de pedido', 'Seguimiento de carrito', 'Encuesta de satisfacción', 'Alta de cliente nuevo']) {
        await create(`/api/v1/projects/${ventas.id}/flows`, { name });
      }
    },
    { orgId: org.id },
  );
}

async function signIn(page: Page) {
  await page.goto('/login');
  await page.getByLabel(/^(Email|Correo)/).fill(account.email);
  await page.getByLabel(/^(Password|Contraseña)/).fill(account.password);
  await page.getByRole('button', { name: /^(Sign in|Iniciar sesión)$/ }).click();
  await page.waitForURL(/\/orgs\//);
  await seed(page);
}

/** Opens one of the menus of the sidebar: `account` (the last button that opens a menu), `organization` (the first) or `help`. */
async function openMenu(page: Page, which: 'account' | 'organization' | 'help') {
  const sidebar = page.getByRole('navigation', { name: /^(Main navigation|Navegación principal)$/ }).first();
  if (which === 'help') await sidebar.getByRole('button', { name: /^(Help|Ayuda)$/ }).click();
  else await sidebar.locator('button[aria-haspopup="menu"]').filter({ hasNot: page.locator('[data-nothing]') })[which === 'account' ? 'last' : 'first']().click();
  await expect(page.getByRole('menu')).toBeVisible();
}

/** One finished run of the sample flow, so the logs have something to show. */
async function mockRuns(page: Page) {
  const execution = '0195f1a0-0000-7000-8000-0000000000e1';
  const json = (body: unknown) => ({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  const base = Date.now() - 3 * 60_000 - 2400;
  const at = (offset: number) => new Date(base + offset).toISOString();
  await page.route('**/api/v1/projects/*/executions**', (route) =>
    route.fulfill(
      json({
        items: [
          {
            id: execution,
            flowId: '01930000-0000-7000-8000-00000000ed5a',
            projectId: '0195f1a0-0000-7000-8000-0000000000d1',
            flowVersion: 1,
            mode: 'MANUAL',
            status: 'SUCCEEDED',
            createdAt: at(0),
            startedAt: at(0),
            finishedAt: at(2400),
            dataHidden: false,
          },
        ],
      }),
    ),
  );
  const nodes = [
    ['webhook', 0, 4],
    ['validate', 4, 7],
    ['inventory', 7, 412],
    ['slack', 412, 1612],
    ['done', 1612, 1613],
  ] as const;
  await page.route('**/api/v1/executions/*/nodes**', (route) =>
    route.fulfill(
      json({
        items: nodes.map(([nodeId, from, to], index) => ({
          id: `0195f1a0-0000-7000-8000-0000000001${String(index).padStart(2, '0')}`,
          executionId: execution,
          nodeId,
          status: 'SUCCEEDED',
          attemptCount: 1,
          startedAt: at(from),
          finishedAt: at(to),
        })),
      }),
    ),
  );
  await page.route('**/api/v1/executions/*/data**', (route) =>
    route.fulfill(
      json({
        items: [
          {
            nodeExecutionId: '0195f1a0-0000-7000-8000-000000000103',
            nodeId: 'slack',
            side: 'INPUT',
            port: 'main',
            itemCount: 1,
            dataItems: [
              {
                output: 'Hello Camila! We have the Andes jacket in size M, olive green, at $289,900.',
                customer: { name: 'Camila Rojas', phone: '573104567890' },
                message: { id: 'wamid.HBgMNTczMTA0NTY3ODkw', text: 'Hi, do you have the Andes jacket in size M?', receivedAt: '2026-10-15T10:41:07-05:00' },
                intent: 'buy',
                needsAdvisor: false,
              },
            ],
          },
          { nodeExecutionId: '0195f1a0-0000-7000-8000-000000000103', nodeId: 'slack', side: 'OUTPUT', port: 'main', itemCount: 1, dataItems: [{ status: 200, body: { ok: true, ts: '1760524861.001' } }] },
        ],
      }),
    ),
  );
}

async function openEditor(page: Page) {
  await mockDraftApi(page, { published: true });
  await mockRuns(page);
  await page.goto(EDITOR_FLOW);
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  await page.waitForSelector('[data-fitted="true"]');
}

const screens: Screen[] = [
  {
    name: 'sign-in',
    mockup: { id: 'login', variant: 'normal' },
    product: async (page) => {
      await page.goto('/login');
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    },
  },
  {
    name: 'sign-in-error',
    mockup: { id: 'login', variant: 'error' },
    product: async (page) => {
      await page.goto('/login');
      await page.getByLabel(/^(Email|Correo)/).fill(account.email);
      await page.getByLabel(/^(Password|Contraseña)/).fill('not-the-password-1234');
      await page.getByRole('button', { name: /^(Sign in|Iniciar sesión)$/ }).click();
      await expect(page.getByRole('alert')).toBeVisible();
    },
  },
  {
    name: 'sign-in-forgot-password',
    mockup: { id: 'login', variant: 'forgot' },
    product: async (page) => {
      await page.goto('/login?step=forgot-password');
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    },
  },
  {
    name: 'shell',
    mockup: { id: 'overview' },
    product: async (page) => {
      await signIn(page);
      await page.goto(`/orgs/${org.slug}`);
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    },
  },
  {
    name: 'shell-menus',
    mockup: { id: 'sidebar', variant: 'menus' },
    devices: ['desktop'],
    product: async (page) => {
      await signIn(page);
      await page.goto(`/orgs/${org.slug}`);
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      await openMenu(page, 'account');
    },
  },
  {
    name: 'shell-org-menu',
    mockup: { id: 'sidebar', variant: 'menus' },
    devices: ['desktop'],
    product: async (page) => {
      await signIn(page);
      await page.goto(`/orgs/${org.slug}`);
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      await openMenu(page, 'organization');
    },
  },
  {
    name: 'shell-help-menu',
    mockup: { id: 'sidebar', variant: 'menus' },
    devices: ['desktop'],
    product: async (page) => {
      await signIn(page);
      await page.goto(`/orgs/${org.slug}`);
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      await openMenu(page, 'help');
    },
  },
  {
    name: 'shell-collapsed',
    mockup: { id: 'sidebar', variant: 'collapsed' },
    devices: ['desktop'],
    product: async (page) => {
      await signIn(page);
      await page.goto(`/orgs/${org.slug}`);
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      await page.keyboard.press('[');
    },
  },
  ...(['blank', 'template', 'import', 'ai'] as const).map(
    (mode): Screen => ({
      name: `create-flow-${mode}`,
      mockup: { id: 'newflow', variant: mode },
      devices: ['desktop'],
      product: async (page) => {
        await signIn(page);
        // Before the chooser the address knew only `?create=true`, which opened the name dialog.
        await page.goto(`/orgs/${org.slug}/projects/ventas-whatsapp/flows?create=${label === 'before' ? 'true' : mode}`);
        await expect(page.getByRole('dialog')).toBeVisible();
      },
    }),
  ),
  {
    name: 'editor',
    mockup: { id: 'editor', variant: 'logs' },
    product: async (page, device) => {
      await openEditor(page);
      if (device === 'desktop') await page.waitForSelector('[data-fitted="true"]');
    },
  },
  {
    name: 'editor-palette',
    mockup: { id: 'editor', variant: 'palette' },
    devices: ['desktop'],
    product: async (page) => {
      await openEditor(page);
      // A is the keyboard way to add a step after the focused one: the list then suggests what usually follows.
      await page.locator('.react-flow__node[data-id="validate"]').click();
      await page.keyboard.press('a');
      await expect(page.locator('[data-node-palette]')).toBeVisible();
    },
  },
  {
    name: 'editor-palette-empty',
    mockup: { id: 'editor', variant: 'palette' },
    devices: ['desktop'],
    product: async (page) => {
      await openEditor(page);
      await page.locator('.react-flow__node[data-id="webhook"]').click();
      await page.keyboard.press('a');
      await page.getByRole('combobox', { name: /^(Search nodes|Buscar nodos)/ }).fill('quickbooks');
      await expect(page.locator('[data-palette-no-match]')).toBeVisible();
    },
  },
  {
    name: 'editor-logs',
    mockup: { id: 'editor', variant: 'logs' },
    devices: ['desktop'],
    product: async (page) => {
      await openEditor(page);
      await page.getByRole('button', { name: /^(Show or hide the logs|Mostrar u ocultar los registros)/ }).click();
      await page.getByRole('button', { name: /^Slack/ }).first().click();
    },
  },
  {
    name: 'editor-details',
    mockup: { id: 'ndv' },
    devices: ['desktop'],
    product: async (page) => {
      await openEditor(page);
      await page.locator('.react-flow__node[data-id="slack"]').dblclick();
      await expect(page.getByRole('button', { name: /Use customer\.phone in a parameter|Usar customer\.phone en un parámetro/ })).toBeVisible();
    },
  },
  {
    name: 'editor-details-chips',
    mockup: { id: 'ndv' },
    devices: ['desktop'],
    product: async (page) => {
      await openEditor(page);
      await page.locator('.react-flow__node[data-id="slack"]').dblclick();
      const field = page.getByRole('button', { name: /Use customer\.phone in a parameter|Usar customer\.phone en un parámetro/ });
      await field.dragTo(page.getByRole('textbox', { name: /URL/ }));
      await expect(page.locator('[data-expression-chips-field]').first()).toBeVisible();
    },
  },
  {
    name: 'editor-details-retry',
    mockup: { id: 'ndv' },
    devices: ['desktop'],
    product: async (page) => {
      await openEditor(page);
      await page.locator('.react-flow__node[data-id="slack"]').dblclick();
      await page.getByRole('tab', { name: /Retries and safety|Reintentos y seguridad/ }).click();
    },
  },
  {
    name: 'overview',
    mockup: { id: 'overview' },
    product: async (page) => {
      await signIn(page);
      await page.goto(`/orgs/${org.slug}/overview`);
      await expect(page.getByRole('link', { name: 'Agente de ventas' })).toBeVisible();
    },
  },
  {
    name: 'overview-credentials',
    mockup: { id: 'overview' },
    product: async (page) => {
      await signIn(page);
      await page.goto(`/orgs/${org.slug}/overview?tab=credentials`);
      await expect(page.locator('[data-coming-soon]')).toBeVisible();
    },
  },
  {
    name: 'project',
    mockup: { id: 'project', variant: 'ventas' },
    product: async (page) => {
      await signIn(page);
      await page.goto(`/orgs/${org.slug}/projects/ventas-whatsapp/flows`);
      await expect(page.getByRole('link', { name: 'Agente de ventas' })).toBeVisible();
    },
  },
  {
    name: 'project-empty',
    mockup: { id: 'project', variant: 'ops' },
    product: async (page) => {
      await signIn(page);
      await page.goto(`/orgs/${org.slug}/projects/operaciones/flows`);
      await expect(page.locator('[data-mascot-pose="thinking"]')).toBeVisible();
    },
  },
  {
    name: 'executions-list',
    mockup: { id: 'executions' },
    product: async (page) => {
      await signIn(page);
      await mockExecutions(page);
      await page.goto(`/orgs/${org.slug}/executions`);
      await expect(page.getByRole('list', { name: /^(Runs|Ejecuciones)$/ })).toBeVisible();
    },
  },
  {
    name: 'executions-failed',
    mockup: { id: 'executions' },
    product: async (page) => {
      await signIn(page);
      const runs = await mockExecutions(page);
      await page.goto(`/orgs/${org.slug}/executions/${runs.failedId}`);
      await expect(page.getByRole('list', { name: /^(Steps of the run|Pasos de la ejecución)$/ })).toBeVisible();
      await expect(page.locator('[data-plain-error]')).toBeVisible();
    },
  },
  {
    name: 'executions-review',
    mockup: { id: 'executions' },
    product: async (page) => {
      await signIn(page);
      const runs = await mockExecutions(page);
      await page.goto(`/orgs/${org.slug}/executions/${runs.reviewId}`);
      await expect(page.locator('[data-uncertain-panel]')).toBeVisible();
    },
  },
  {
    name: 'settings',
    mockup: { id: 'settings', variant: 'apariencia' },
    product: async (page) => {
      await signIn(page);
      await page.goto('/settings/appearance');
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    },
  },
];

const selected = screens.filter((screen) => only === undefined || only.includes(screen.name));

test.describe('product', () => {
  test.skip(mockupUrl !== undefined, 'The mockup is being captured.');
  for (const screen of selected) {
    for (const device of devices) {
      if (screen.devices !== undefined && !screen.devices.includes(device.name)) continue;
      for (const theme of themes) {
        test(`${screen.name} ${device.name} ${theme}`, async ({ page }) => {
          await page.setViewportSize({ width: device.width, height: device.height });
          await page.addInitScript(
            ({ value }) => {
              window.localStorage.setItem('orcon.theme', value);
              window.localStorage.setItem('orcon.locale', 'es');
            },
            { value: theme },
          );
          if (!screen.name.startsWith('editor')) await installApiDouble(page, { demo: true });
          await screen.product(page, device.name);
          await page.waitForTimeout(400);
          const dir = join(out, label);
          mkdirSync(dir, { recursive: true });
          await page.screenshot({ path: join(dir, `${screen.name}-${device.name}-${theme}.png`) });
        });
      }
    }
  }
});

test.describe('mockup', () => {
  test.skip(mockupUrl === undefined, 'Set ORCON_MOCKUP_URL to capture the mockup.');
  for (const screen of selected) {
    for (const device of devices) {
      if (screen.devices !== undefined && !screen.devices.includes(device.name)) continue;
      for (const theme of themes) {
        test(`${screen.name} ${device.name} ${theme}`, async ({ page }) => {
          // The mockup scales its stage to fit the window: a window larger than the device keeps the scale at 1.
          await page.setViewportSize({ width: 1600, height: 1200 });
          await page.goto(`${mockupUrl ?? ''}#${screen.mockup.id}${screen.mockup.variant === undefined ? '' : `:${screen.mockup.variant}`}`);
          await page.evaluate(
            ({ value, phone }) => {
              document.body.classList.add('no-notes');
              document.documentElement.dataset['theme'] = value;
              const button = document.querySelector<HTMLButtonElement>(`[data-dev="${phone ? 'phone' : 'desktop'}"]`);
              button?.click();
            },
            { value: theme, phone: device.name === 'phone' },
          );
          await page.waitForTimeout(500);
          const dir = join(out, 'mockup');
          mkdirSync(dir, { recursive: true });
          await page.locator('#device').screenshot({ path: join(dir, `${screen.name}-${device.name}-${theme}.png`) });
        });
      }
    }
  }
});
