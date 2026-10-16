// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http } from 'msw';
import { describe, expect, it, vi } from 'vitest';

import { listedFlow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';
import { FIRST_USEFUL_MARK } from '../../../app/perf/marks';

const greeting = { level: 1, name: /^Good (morning|afternoon|evening), Ana$/ } as const;
const address = '/orgs/acme/overview';

/** Everything the home page reads before its own list: the person, the organization and the projects. */
const beforeHome = (summary = productApi.activitySummary()) => [signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]), summary];

describe('OverviewScreen', () => {
  it('loading', async () => {
    server.use(...beforeHome(), http.get(`/api/v1/orgs/${acme.id}/flows`, () => new Promise<never>(() => undefined)));
    renderRoute(address);
    expect(await screen.findByRole('status', { name: 'Loading' })).toHaveAttribute('aria-busy', 'true');
  });

  it('empty', async () => {
    server.use(...beforeHome(), productApi.orgFlows([]));
    renderRoute(address);
    expect(await screen.findByRole('heading', { name: 'No flows yet' })).toBeVisible();
    expect(document.querySelector('[data-mascot-pose="thinking"]')).not.toBeNull();
    expect(screen.getAllByRole('button', { name: 'Create flow' }).length).toBeGreaterThan(0);
  });

  it('error', async () => {
    server.use(...beforeHome(), http.get(`/api/v1/orgs/${acme.id}/flows`, () => problem(500, 'Internal Server Error', 'internal')));
    renderRoute(address);
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
  });

  it('forbidden', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.activitySummary(), http.get(`/api/v1/orgs/${acme.id}/projects`, () => problem(403, 'Forbidden', 'forbidden')));
    renderRoute(address);
    expect(await screen.findByRole('heading', { name: /permission/i })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
  });

  it('data', async () => {
    const support = { ...project, id: '0195f1a0-0000-7000-8000-0000000000d2', name: 'Soporte', handle: 'soporte', slug: 'soporte' };
    const other = { ...listedFlow, id: '0195f1a0-0000-7000-8000-0000000000e2', projectId: support.id, name: 'Escalate to a person', activationState: 'INACTIVE', activeVersion: undefined, lastExecution: undefined, executionCountLast24h: 0 };
    server.use(
      signedIn.currentUser,
      signedIn.organizations([acme]),
      productApi.projects([project, support]),
      productApi.activitySummary(),
      productApi.orgFlows([listedFlow, other]),
    );
    renderRoute(address);
    expect(await screen.findByRole('heading', greeting)).toBeVisible();

    // The five figures, from the summary of the server.
    const figures = await screen.findByRole('list', { name: 'Activity of the organization' });
    expect(within(figures).getAllByRole('listitem')).toHaveLength(5);
    expect(within(figures).getByText('Active flows')).toBeVisible();
    expect(within(figures).getByText('of 18')).toBeVisible();
    expect(within(figures).getByText('Needs review')).toBeVisible();
    expect(within(figures).getByRole('img', { name: /last 24 hours/ })).toBeVisible();

    // The tabs: flows with their count, credentials and executions without one.
    const tabs = screen.getByRole('navigation', { name: 'What to show' });
    expect(within(tabs).getByRole('link', { name: /Flows/ })).toHaveAttribute('aria-current', 'page');
    expect(within(tabs).getByRole('link', { name: /Credentials/ })).toBeVisible();

    // A row names the project and shows the last run as icon and word; a flow that never ran says so.
    const row = (await screen.findByRole('link', { name: 'Order notifications' })).closest('li') as HTMLElement;
    expect(row).toHaveTextContent('Ventas');
    expect(row).toHaveTextContent('Succeeded');
    expect(within(row).getByRole('switch', { name: 'Active: Order notifications' })).toBeChecked();
    const idle = screen.getByRole('link', { name: 'Escalate to a person' }).closest('li') as HTMLElement;
    expect(idle).toHaveTextContent('Never run');
    expect(screen.getByRole('link', { name: 'Escalate to a person' })).toHaveAttribute('href', '/orgs/acme/projects/soporte/flows/0195f1a0-0000-7000-8000-0000000000e2');
  });

  it('hides the figures when the summary operation is unavailable', async () => {
    server.use(...beforeHome(productApi.activitySummary(() => problem(503, 'Service Unavailable', 'service-unavailable'))), productApi.orgFlows([listedFlow]));
    renderRoute(address);
    await screen.findByRole('link', { name: 'Order notifications' });
    expect(screen.queryByRole('list', { name: 'Activity of the organization' })).toBeNull();
    expect(screen.queryByText('Active flows')).toBeNull();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('shows the time saved note with the organization minutes', async () => {
    server.use(...beforeHome(), productApi.orgFlows([listedFlow]));
    renderRoute(address);
    expect(await screen.findByText('Estimated with 3 min per task')).toBeVisible();
    // 1860 minutes are 31 hours.
    expect(screen.getByText('31 h')).toBeVisible();
  });

  it('keeps the tab in the URL', async () => {
    server.use(...beforeHome(), productApi.orgFlows([listedFlow]));
    const { router } = renderRoute(address);
    const tabs = await screen.findByRole('navigation', { name: 'What to show' });
    await userEvent.click(within(tabs).getByRole('link', { name: /Credentials/ }));
    await waitFor(() => {
      expect(router.state.location.search).toEqual({ tab: 'credentials' });
    });
    expect(await screen.findByText('Coming soon')).toBeVisible();
    // A reload on that address shows the same tab.
    renderRoute('/orgs/acme/overview?tab=executions');
    expect(await screen.findByRole('heading', { name: 'Executions of every project' })).toBeVisible();
  });

  it('opens a flow with a click on its row', async () => {
    server.use(...beforeHome(), productApi.orgFlows([listedFlow]));
    const { router } = renderRoute(address);
    await userEvent.click(await screen.findByRole('link', { name: 'Order notifications' }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/orgs/acme/projects/ventas/flows/${listedFlow.id}`);
    });
  });

  it('says there are no projects yet and offers to create one', async () => {
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([]), productApi.activitySummary());
    renderRoute(address);
    expect(await screen.findByRole('heading', { name: 'No projects yet' })).toBeVisible();
    // One in the sidebar, next to the Projects heading, and one here, with the explanation.
    expect(screen.getAllByRole('button', { name: 'Create a project' }).length).toBeGreaterThan(1);
  });

  it('marks the first useful screen', async () => {
    const mark = vi.spyOn(performance, 'mark');
    server.use(...beforeHome(), productApi.orgFlows([listedFlow]));
    renderRoute(address);
    await screen.findByRole('heading', greeting);
    expect(mark).toHaveBeenCalledWith(FIRST_USEFUL_MARK, { detail: { route: '/orgs/$orgSlug/overview' } });
    mark.mockRestore();
  });

  it('starts a new flow in the first project from the button of the page', async () => {
    server.use(...beforeHome(), productApi.orgFlows([listedFlow]));
    const { router } = renderRoute(address);
    await screen.findByRole('heading', greeting);
    await userEvent.click(screen.getAllByRole('button', { name: 'Create flow' })[0] as HTMLElement);
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/orgs/acme/projects/ventas/flows');
    });
    expect(await screen.findByRole('dialog', { name: 'Create flow' })).toBeVisible();
  });
});
