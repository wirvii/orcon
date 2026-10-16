// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http } from 'msw';
import { describe, expect, it } from 'vitest';

import { executionApi, otherRun, run } from '../../../../test/execution-fixtures';
import { productApi, flow, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';

const address = '/orgs/acme/projects/ventas/executions';

/** Everything the runs page reads before its own list: the person, the organization and the project. */
const beforeRuns = (flows: readonly object[] = [flow]) => [signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]), productApi.orgFlows(flows)];

describe('ExecutionListScreen', () => {
  it('loading', async () => {
    server.use(...beforeRuns(), http.get(`/api/v1/projects/${project.id}/executions`, () => new Promise(() => undefined)));
    renderRoute(address);
    expect(await screen.findByRole('status', { name: 'Loading' })).toBeVisible();
  });

  it('empty', async () => {
    server.use(...beforeRuns(), executionApi.list([[]]));
    renderRoute(address);
    expect(await screen.findByRole('heading', { name: 'No runs yet' })).toBeVisible();
  });

  it('error', async () => {
    server.use(...beforeRuns(), http.get(`/api/v1/projects/${project.id}/executions`, () => problem(500, 'internal', 'internal')));
    renderRoute(address);
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
  });

  it('unavailable: a calm state, no server title and no request id', async () => {
    server.use(
      ...beforeRuns(),
      http.get(`/api/v1/projects/${project.id}/executions`, () =>
        problem(503, 'Service unavailable', 'service-unavailable', { 'Retry-After': '30' }),
      ),
    );
    renderRoute(address);
    expect(await screen.findByRole('heading', { name: 'Run history will be available soon' })).toBeVisible();
    expect(screen.queryByText('Service unavailable')).toBeNull();
    expect(screen.queryByText('Request ID')).toBeNull();
  });

  it('forbidden', async () => {
    server.use(...beforeRuns(), http.get(`/api/v1/projects/${project.id}/executions`, () => problem(403, 'forbidden', 'forbidden')));
    renderRoute(address);
    expect(await screen.findByText('You do not have permission to see this')).toBeVisible();
  });

  it('data, with the next page on request and the flow filter from the address', async () => {
    server.use(...beforeRuns(), executionApi.list([[run], [otherRun]]));
    renderRoute(address);
    const first = await screen.findByRole('link', { name: 'Order notifications' });
    expect(first).toHaveAttribute('href', `${address}/0195f1a0-0000-7000-8000-0000000000f1`);
    await userEvent.click(screen.getByRole('button', { name: 'Load more runs' }));
    await waitFor(() => {
      expect(screen.getAllByRole('link', { name: 'Order notifications' })).toHaveLength(2);
    });
    expect(screen.getAllByRole('link', { name: 'Order notifications' })[1]).toHaveAttribute('href', `${address}/0195f1a0-0000-7000-8000-0000000000f2`);
    expect(screen.queryByRole('button', { name: 'Load more runs' })).toBeNull();
  });

  it('tells runs that started together apart: the flow name, the start time and the random end of the identifier', async () => {
    server.use(...beforeRuns(), executionApi.list([[run, otherRun]]));
    renderRoute(address);
    await screen.findAllByRole('link', { name: 'Order notifications' });
    // The two runs share the first characters of their identifiers (time-ordered), so the end is what differs.
    expect(screen.getByText(/Run .000000f1/)).toBeVisible();
    expect(screen.getByText(/Run .000000f2/)).toBeVisible();
    const times = Array.from(document.querySelectorAll('time')).map((element) => element.getAttribute('datetime'));
    expect(times).toEqual(expect.arrayContaining(['2026-10-14T09:00:01.000Z', '2026-10-14T08:00:00.000Z']));
  });

  it('names a run of a deleted flow instead of hiding it', async () => {
    const gone = { ...run, flowId: '0195f1a0-0000-7000-8000-0000000000e9' };
    server.use(...beforeRuns(), executionApi.list([[gone, otherRun]]));
    renderRoute(address);
    expect(await screen.findByRole('link', { name: 'Deleted flow' })).toHaveAttribute('href', `${address}/${gone.id}`);
    expect(screen.getByRole('link', { name: 'Order notifications' })).toBeVisible();
  });

  it('shows the runs of one flow only when the address names it', async () => {
    let requested: string | null = null;
    server.use(
      ...beforeRuns(),
      http.get(`/api/v1/projects/${project.id}/executions`, ({ request }) => {
        requested = new URL(request.url).searchParams.get('flowId');
        return Response.json({ items: [run] });
      }),
    );
    renderRoute(`${address}?flowId=${flow.id}`);
    expect(await screen.findByRole('link', { name: 'Order notifications' })).toBeVisible();
    expect(requested).toBe(flow.id);
    expect(screen.getByRole('link', { name: 'Show every run' })).toBeVisible();
  });
});
