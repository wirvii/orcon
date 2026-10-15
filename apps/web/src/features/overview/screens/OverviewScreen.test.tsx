// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it, vi } from 'vitest';

import { flow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';
import { FIRST_USEFUL_MARK } from '../../../app/perf/marks';

const greeting = { level: 1, name: /^Good (morning|afternoon|evening), Ana$/ } as const;

describe('the home page of an organization', () => {
  it('greets the person by first name and lists the flows of every project, most recent first', async () => {
    const support = { ...project, id: '0195f1a0-0000-7000-8000-0000000000d2', name: 'Soporte', handle: 'soporte', slug: 'soporte' };
    const newer = { ...flow, id: '0195f1a0-0000-7000-8000-0000000000e2', projectId: support.id, name: 'Escalate to a person', activationState: 'ACTIVE', updatedAt: '2026-10-14T08:00:00Z' };
    server.use(
      signedIn.currentUser,
      signedIn.organizations([acme]),
      productApi.projects([project, support]),
      productApi.flows([flow]),
      http.get(`/api/v1/projects/${support.id}/flows`, () => HttpResponse.json({ items: [newer] })),
    );
    renderRoute('/orgs/acme/overview');
    expect(await screen.findByRole('heading', greeting)).toBeVisible();
    const list = (await screen.findByRole('heading', { level: 2, name: 'Flows' })).closest('section') as HTMLElement;
    const rows = await within(list).findAllByRole('link');
    expect(rows.map((row) => row.textContent)).toEqual(['Escalate to a personSoporte', 'Order notificationsVentas']);
    // A row is one link to the flow; its state is a word, not only a color.
    expect(rows[0]).toHaveAttribute('href', '/orgs/acme/projects/soporte/flows/0195f1a0-0000-7000-8000-0000000000e2');
    expect(within(list).getByText('Active')).toBeVisible();
    expect(within(list).getByText('Inactive')).toBeVisible();
  });

  it('opens a flow with a click on its row', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]), productApi.flows([flow]));
    const { router } = renderRoute('/orgs/acme/overview');
    await userEvent.click(await screen.findByRole('link', { name: /Order notifications/ }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/orgs/acme/projects/ventas/flows/${flow.id}`);
    });
  });

  it('shows the placeholders while the flows load', async () => {
    server.use(
      signedIn.currentUser,
      signedIn.organizations([acme]),
      productApi.projects([project]),
      http.get(`/api/v1/projects/${project.id}/flows`, () => new Promise<never>(() => undefined)),
    );
    renderRoute('/orgs/acme/overview');
    expect(await screen.findByRole('status', { name: 'Loading flows' })).toHaveAttribute('aria-busy', 'true');
  });

  it('says there are no flows yet and offers the first step', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]), productApi.flows([]));
    renderRoute('/orgs/acme/overview');
    expect(await screen.findByRole('heading', { name: 'No flows yet' })).toBeVisible();
    expect(screen.getAllByRole('button', { name: 'Create flow' }).length).toBeGreaterThan(0);
  });

  it('says there are no projects yet and offers to create one', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([]));
    renderRoute('/orgs/acme/overview');
    expect(await screen.findByRole('heading', { name: 'No projects yet' })).toBeVisible();
    // One in the sidebar, next to the Projects heading, and one here, with the explanation.
    expect(screen.getAllByRole('button', { name: 'Create a project' }).length).toBeGreaterThan(1);
  });

  it('shows a problem with a way to try again when the flows cannot be read', async () => {
    server.use(
      signedIn.currentUser,
      signedIn.organizations([acme]),
      productApi.projects([project]),
      http.get(`/api/v1/projects/${project.id}/flows`, () => problem(500, 'Internal Server Error', 'internal')),
    );
    renderRoute('/orgs/acme/overview');
    expect(await screen.findByRole('alert')).toBeVisible();
  });

  it('shows the permission message, without a retry, when the person may not read the projects', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), http.get(`/api/v1/orgs/${acme.id}/projects`, () => problem(403, 'Forbidden', 'forbidden')));
    renderRoute('/orgs/acme/overview');
    expect(await screen.findByRole('heading', { name: /permission/i })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
  });

  it('marks the first useful screen', async () => {
    const mark = vi.spyOn(performance, 'mark');
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]), productApi.flows([flow]));
    renderRoute('/orgs/acme/overview');
    await screen.findByRole('heading', greeting);
    expect(mark).toHaveBeenCalledWith(FIRST_USEFUL_MARK, { detail: { route: '/orgs/$orgSlug/overview' } });
    mark.mockRestore();
  });

  it('starts a new flow in the first project from the button of the page', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]), productApi.flows([flow]));
    const { router } = renderRoute('/orgs/acme/overview');
    await screen.findByRole('heading', greeting);
    await userEvent.click(screen.getAllByRole('button', { name: 'Create flow' })[0] as HTMLElement);
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/orgs/acme/projects/ventas/flows');
    });
    expect(await screen.findByRole('dialog')).toBeVisible();
  });
});
