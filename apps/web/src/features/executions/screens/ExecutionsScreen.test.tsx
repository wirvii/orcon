// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http } from 'msw';
import { describe, expect, it } from 'vitest';

import { executionApi, otherRun, reviewRun, run } from '../../../../test/execution-fixtures';
import { flow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';

const address = '/orgs/acme/executions';
const listUrl = `/api/v1/orgs/${acme.id}/executions`;

/** Everything the page reads before its own list: the person, the organization, the projects and the flows that name the rows. */
const beforeRuns = (flows: readonly object[] = [flow]) => [signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]), productApi.orgFlows(flows)];

describe('ExecutionsScreen', () => {
  it('loading', async () => {
    server.use(...beforeRuns(), http.get(listUrl, () => new Promise(() => undefined)));
    renderRoute(address);
    expect(await screen.findByRole('status', { name: 'Loading' })).toBeVisible();
  });

  it('empty', async () => {
    server.use(...beforeRuns(), executionApi.list([[]]));
    renderRoute(address);
    expect(await screen.findByRole('heading', { name: 'No runs yet' })).toBeVisible();
  });

  it('error', async () => {
    server.use(...beforeRuns(), http.get(listUrl, () => problem(500, 'internal', 'internal')));
    renderRoute(address);
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
  });

  it('forbidden', async () => {
    server.use(...beforeRuns(), http.get(listUrl, () => problem(403, 'forbidden', 'forbidden')));
    renderRoute(address);
    expect(await screen.findByText('You do not have permission to see this')).toBeVisible();
  });

  it('unavailable: a calm state, no server title and no request id', async () => {
    server.use(...beforeRuns(), http.get(listUrl, () => problem(503, 'Service unavailable', 'service-unavailable', { 'Retry-After': '30' })));
    renderRoute(address);
    expect(await screen.findByRole('heading', { name: 'Run history will be available soon' })).toBeVisible();
    expect(screen.queryByText('Service unavailable')).toBeNull();
    expect(screen.queryByText('Request ID')).toBeNull();
  });

  it('data', async () => {
    server.use(...beforeRuns(), executionApi.list([[run, reviewRun], [otherRun]]));
    renderRoute(address);
    const failed = await screen.findByRole('link', { name: /Order notifications.*Run #000000f1/ });
    expect(failed).toHaveAttribute('href', `${address}/${run.id}`);
    // A run with an operation of unknown outcome says so in words and is not drawn as a success, though the engine finished it.
    const review = screen.getByRole('link', { name: /Order notifications.*Run #000000f3/ });
    const reviewRow = review.closest('li');
    expect(reviewRow).toHaveTextContent('Needs review');
    expect(reviewRow).not.toHaveTextContent('Succeeded');
    expect(failed.closest('li')).toHaveTextContent('Failed');
    // The next page comes on request.
    await userEvent.click(screen.getByRole('button', { name: 'Load more runs' }));
    await waitFor(() => {
      expect(screen.getAllByRole('link', { name: /Order notifications/ })).toHaveLength(3);
    });
    expect(screen.queryByRole('button', { name: 'Load more runs' })).toBeNull();
  });

  it('names a run of a deleted flow instead of hiding it', async () => {
    const gone = { ...run, flowId: '0195f1a0-0000-7000-8000-0000000000e9' };
    server.use(...beforeRuns(), executionApi.list([[gone, otherRun]]));
    renderRoute(address);
    expect(await screen.findByRole('link', { name: /Deleted flow/ })).toHaveAttribute('href', `${address}/${gone.id}`);
    expect(screen.getByRole('link', { name: /Order notifications/ })).toBeVisible();
  });

  it('keeps filters in the URL', async () => {
    const seen: URLSearchParams[] = [];
    server.use(...beforeRuns(), executionApi.list([[run, otherRun]], seen));
    const { router } = renderRoute(`${address}?show=failed&range=7d&flowId=${flow.id}&projectId=${project.id}`);
    await screen.findAllByRole('link', { name: /Order notifications/ });
    // What the address says is what the server is asked for.
    const first = seen[0];
    expect(first?.get('status')).toBe('FAILED');
    expect(first?.get('flowId')).toBe(flow.id);
    expect(first?.get('projectId')).toBe(project.id);
    expect(Date.now() - Date.parse(first?.get('createdAfter') ?? '')).toBeGreaterThan(6.9 * 24 * 3_600_000);
    expect(screen.getByRole('button', { name: 'Failed' })).toHaveAttribute('aria-pressed', 'true');
    // A change of a filter goes to the address, replacing the entry so Back does not step through every choice.
    await userEvent.click(screen.getByRole('button', { name: 'All' }));
    await waitFor(() => {
      expect(router.state.location.search).not.toHaveProperty('show');
    });
    expect(router.state.location.search).toMatchObject({ range: '7d', flowId: flow.id, projectId: project.id });
  });

  it('a malformed filter in the address is no filter', async () => {
    const seen: URLSearchParams[] = [];
    server.use(...beforeRuns(), executionApi.list([[run]], seen));
    renderRoute(`${address}?show=everything&flowId=nope&range=forever`);
    await screen.findAllByRole('link', { name: /Order notifications/ });
    expect(seen[0]?.get('status')).toBeNull();
    expect(seen[0]?.get('flowId')).toBeNull();
    expect(seen[0]?.get('createdAfter')).toBeNull();
  });

  it('needs review filter uses the uncertain parameter', async () => {
    const seen: URLSearchParams[] = [];
    server.use(...beforeRuns(), executionApi.list([[run, reviewRun]], seen));
    const { router } = renderRoute(address);
    await screen.findAllByRole('link', { name: /Order notifications/ });
    expect(seen[0]?.get('uncertain')).toBeNull();
    await userEvent.click(screen.getByRole('button', { name: 'Needs review' }));
    await waitFor(() => {
      expect(seen.some((query) => query.get('uncertain') === 'true')).toBe(true);
    });
    expect(router.state.location.search).toMatchObject({ show: 'review' });
    // The server answered only the run that needs review, and the status filter was not used for it.
    expect(await screen.findAllByRole('link', { name: /Order notifications/ })).toHaveLength(1);
    expect(seen.find((query) => query.get('uncertain') === 'true')?.get('status')).toBeNull();
  });

  it('search accepts an execution id only', async () => {
    const seen: URLSearchParams[] = [];
    server.use(...beforeRuns(), executionApi.list([[run, otherRun]], seen));
    renderRoute(address);
    await screen.findAllByRole('link', { name: /Order notifications/ });
    const box = screen.getByRole('searchbox', { name: 'Search by run identifier' });
    // Words, phone numbers and half an identifier are not searched: the box says what it takes and nothing is asked.
    await userEvent.type(box, '+57 300 1234567');
    expect(await screen.findByText(/Paste the whole identifier of a run/)).toBeVisible();
    expect(seen.every((query) => query.get('id') === null)).toBe(true);
    await userEvent.clear(box);
    await waitFor(() => {
      expect(screen.queryByText(/Paste the whole identifier of a run/)).toBeNull();
    });
    // A whole identifier is searched.
    await userEvent.type(box, otherRun.id);
    await waitFor(() => {
      expect(seen.some((query) => query.get('id') === otherRun.id)).toBe(true);
    });
    expect(await screen.findAllByRole('link', { name: /Order notifications/ })).toHaveLength(1);
  });

  it('shows no live chip without list events', async () => {
    server.use(...beforeRuns(), executionApi.list([[run]]));
    renderRoute(address);
    await screen.findAllByRole('link', { name: /Order notifications/ });
    expect(screen.queryByText('Live')).toBeNull();
    // The list is read again on request instead.
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeVisible();
  });

  it('the project tab lists the runs of the project and opens a run on its own page', async () => {
    const seen: URLSearchParams[] = [];
    server.use(...beforeRuns(), executionApi.list([[run]], seen));
    renderRoute('/orgs/acme/projects/ventas/executions');
    const link = await screen.findByRole('link', { name: /Order notifications/ });
    expect(link).toHaveAttribute('href', `/orgs/acme/projects/ventas/executions/${run.id}`);
    expect(seen[0]?.get('projectId')).toBe(project.id);
    // The list of one project does not offer the project filter.
    expect(screen.queryByRole('combobox', { name: 'Project' })).toBeNull();
  });
});
