// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, type Page } from '@playwright/test';

import { mockDraftApi } from '../../test/draft-api';
import { account, installApiDouble, org } from '../journeys/api-double';

/**
 * The screens of the product that the UX checks walk (SPEC-019). Each one opens a page of the production build against
 * a stand-in API and waits until it shows its heading, so a check runs on a real, settled page.
 */

export interface ProductScreen {
  /** The name used in test titles. */
  name: string;
  open: (page: Page) => Promise<void>;
}

/** A signed-in organization with one project that has one flow, created through the same API the screens use. */
async function seed(page: Page): Promise<{ projectHandle: string; flowId: string }> {
  await installApiDouble(page, { signedIn: true, demo: true });
  await page.goto('/login');
  return page.evaluate(async ({ orgId }) => {
    const create = async (path: string, body: unknown) => {
      const response = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
      return (await response.json()) as { id: string; handle?: string };
    };
    const project = await create(`/api/v1/orgs/${orgId}/projects`, { name: 'Ventas WhatsApp' });
    const flow = await create(`/api/v1/projects/${project.id}/flows`, { name: 'Agente de ventas' });
    return { projectHandle: project.handle ?? 'ventas-whatsapp', flowId: flow.id };
  }, { orgId: org.id });
}

async function heading(page: Page, level: 1 | 2, name: string | RegExp) {
  await expect(page.getByRole('heading', { level, name })).toBeVisible();
}

export const screens: ProductScreen[] = [
  {
    name: 'sign-in',
    open: async (page) => {
      await page.goto('/login');
      await heading(page, 1, /^(Sign in|Iniciar sesión)$/);
    },
  },
  {
    name: 'overview',
    open: async (page) => {
      await seed(page);
      await page.goto(`/orgs/${org.slug}/overview`);
      await expect(page.getByRole('link', { name: /Agente de ventas|Sales agent/ })).toBeVisible();
    },
  },
  {
    name: 'project',
    open: async (page) => {
      const { projectHandle } = await seed(page);
      await page.goto(`/orgs/${org.slug}/projects/${projectHandle}/flows`);
      await expect(page.getByRole('link', { name: 'Agente de ventas' })).toBeVisible();
    },
  },
  {
    name: 'project-empty',
    open: async (page) => {
      await seed(page);
      const handle = await page.evaluate(async ({ orgId }) => {
        const response = await fetch(`/api/v1/orgs/${orgId}/projects`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: 'Operaciones' }) });
        return ((await response.json()) as { handle: string }).handle;
      }, { orgId: org.id });
      await page.goto(`/orgs/${org.slug}/projects/${handle}/flows`);
      await expect(page.locator('[data-mascot-pose="thinking"]')).toBeVisible();
    },
  },
  {
    name: 'create-flow',
    open: async (page) => {
      const { projectHandle } = await seed(page);
      await page.goto(`/orgs/${org.slug}/projects/${projectHandle}/flows?create=true`);
      await expect(page.getByRole('dialog', { name: /^(Create flow|Crear flujo)$/ })).toBeVisible();
    },
  },
  {
    name: 'executions',
    open: async (page) => {
      const { projectHandle } = await seed(page);
      await page.goto(`/orgs/${org.slug}/projects/${projectHandle}/executions`);
      await heading(page, 2, /^(Executions|Ejecuciones|Runs)/);
    },
  },
  {
    name: 'credentials',
    open: async (page) => {
      const { projectHandle } = await seed(page);
      await page.goto(`/orgs/${org.slug}/projects/${projectHandle}/credentials`);
      await heading(page, 2, /^(Credentials|Credenciales)$/);
    },
  },
  {
    name: 'settings',
    open: async (page) => {
      await seed(page);
      await page.goto(`/orgs/${org.slug}/settings/appearance`);
      await heading(page, 2, /^(Appearance|Apariencia)$/);
    },
  },
  {
    name: 'editor',
    open: async (page) => {
      await mockDraftApi(page);
      await page.goto('/orgs/acme/projects/ventas/flows/01930000-0000-7000-8000-00000000ed5a');
      await heading(page, 1, 'Order notifications');
      await page.waitForSelector('[data-fitted="true"]');
    },
  },
];

/** What the sign-in page asks for: the account of the stand-in API. */
export const signInAccount = account;
