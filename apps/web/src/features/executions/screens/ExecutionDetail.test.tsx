// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { executionApi, greetingData, greetingStep, otherRun, reviewRun, reviewRunId, run, runId, slackStep } from '../../../../test/execution-fixtures';
import { flow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';

const address = `/orgs/acme/executions/${runId}`;

/** Everything the page reads before the data of the run: the person, the organization, the projects, the flows and the list beside it. */
const beforeRun = () => [
  signedIn.currentUser,
  signedIn.organizations([acme]),
  productApi.projects([project]),
  productApi.orgFlows([flow]),
  executionApi.list([[run, otherRun, reviewRun]]),
];

const failedRun = () => [executionApi.run(() => HttpResponse.json(run)), executionApi.nodes([greetingStep, slackStep]), executionApi.version(), executionApi.data([greetingData])];

describe('ExecutionDetail', () => {
  it('loading', async () => {
    server.use(...beforeRun(), http.get(`/api/v1/executions/${runId}`, () => new Promise(() => undefined)), executionApi.nodes([]));
    renderRoute(address);
    expect((await screen.findAllByRole('status', { name: 'Loading' })).length).toBeGreaterThan(0);
  });

  it('error', async () => {
    server.use(...beforeRun(), executionApi.run(() => problem(500, 'internal', 'internal')), executionApi.nodes([]));
    renderRoute(address);
    expect((await screen.findAllByRole('alert')).length).toBeGreaterThan(0);
  });

  it('header: the flow, the end of the identifier, the status, how it started and the version', async () => {
    server.use(...beforeRun(), ...failedRun());
    renderRoute(address);
    const heading = await screen.findByRole('heading', { level: 2, name: /Order notifications/ });
    expect(heading).toHaveTextContent(/Run #\w*000000f1/);
    const header = heading.closest('header');
    expect(header).toHaveTextContent('Failed');
    expect(header).toHaveTextContent('By hand');
    expect(within(header as HTMLElement).getByText('2')).toBeVisible();
    expect(within(header as HTMLElement).getByText('Ventas')).toBeVisible();
    expect(within(header as HTMLElement).getByText(runId)).toBeVisible();
  });

  it('timeline shows durations and steps that did not run', async () => {
    server.use(...beforeRun(), ...failedRun());
    renderRoute(address);
    const timeline = await screen.findByRole('list', { name: 'Steps of the run' });
    const steps = await within(timeline).findAllByRole('listitem');
    // The steps are named as the flow names them, in the order they ran, and the one the run never reached comes last.
    expect(steps.map((step) => step.getAttribute('data-step'))).toEqual(['greeting', 'slack', 'mark']);
    expect(steps[0]).toHaveTextContent('Greet the customer');
    expect(steps[1]).toHaveTextContent('Notify the team');
    expect(steps[1]).toHaveTextContent('2 sec');
    expect(steps[2]).toHaveTextContent('Mark as notified');
    expect(steps[2]).toHaveTextContent('Did not run');
    // Each step says its status in words for assistive technology, and a step that ran has a bar.
    expect(steps[1]).toHaveTextContent('Failed');
    expect(within(steps[1] as HTMLElement).getByRole('img')).toBeInTheDocument();
    expect(within(steps[2] as HTMLElement).queryByRole('img')).toBeNull();
  });

  it('timeline: a step that ran once per item says how many of them failed', async () => {
    const items = Array.from({ length: 12 }, (_, index) => ({
      ...slackStep,
      id: `0195f1a0-0000-7000-8000-00000000b${String(index).padStart(3, '0')}`,
      status: index < 7 ? 'FAILED' : 'SUCCEEDED',
    }));
    server.use(...beforeRun(), executionApi.run(() => HttpResponse.json(run)), executionApi.nodes(items), executionApi.version(), executionApi.data([]));
    renderRoute(address);
    expect(await screen.findByText('7 of 12 failed')).toBeVisible();
  });

  it('timeline: steps are named by their id when the version cannot be read', async () => {
    server.use(...beforeRun(), executionApi.run(() => HttpResponse.json(run)), executionApi.nodes([greetingStep, slackStep]), executionApi.version(() => problem(404, 'not found', 'not-found')), executionApi.data([]));
    renderRoute(address);
    const timeline = await screen.findByRole('list', { name: 'Steps of the run' });
    expect(await within(timeline).findByText('greeting')).toBeVisible();
    expect(within(timeline).queryByText('Did not run')).toBeNull();
  });

  it('plain error shows the plain title and the original error', async () => {
    server.use(...beforeRun(), ...failedRun());
    renderRoute(address);
    // The failed step opens by itself, with its error in plain words and the original beside it.
    expect(await screen.findByRole('heading', { level: 3, name: 'The step failed' })).toBeVisible();
    expect(screen.getByText('The webhook answered 500.')).toBeVisible();
    const original = screen.getByRole('region', { name: 'Original error' });
    expect(original).toHaveTextContent('node-failed (step: slack)');
  });

  it('hides apply in editor', async () => {
    server.use(...beforeRun(), ...failedRun());
    renderRoute(address);
    await screen.findByRole('heading', { level: 3, name: 'The step failed' });
    // Applying a fix does not exist yet: there is no such button, and no fix is made up.
    expect(screen.queryByRole('button', { name: /apply/i })).toBeNull();
    expect(screen.queryByRole('link', { name: /apply/i })).toBeNull();
    expect(screen.queryByText('How to fix it')).toBeNull();
  });

  it('shows the items of the step that is open and the attempts of each', async () => {
    server.use(...beforeRun(), ...failedRun());
    renderRoute(address);
    const timeline = await screen.findByRole('list', { name: 'Steps of the run' });
    await userEvent.click(await within(timeline).findByRole('button', { name: /Greet the customer/ }));
    await userEvent.click(await screen.findByRole('tab', { name: /^Output/ }));
    expect(await screen.findByRole('region', { name: 'Output of Greet the customer, port main' })).toBeVisible();
    expect(screen.getByText(/Hello, Ana/)).toBeVisible();
    await userEvent.click(screen.getByRole('tab', { name: 'Attempts' }));
    expect(screen.getByText('1 attempt, retries included.')).toBeVisible();
  });

  it('hidden: the items are not asked for and the page says why', async () => {
    let asked = false;
    server.use(
      ...beforeRun(),
      executionApi.run(() => HttpResponse.json({ ...run, dataHidden: true })),
      executionApi.nodes([greetingStep]),
      executionApi.version(),
      http.get(`/api/v1/executions/${runId}/data`, () => {
        asked = true;
        return HttpResponse.json({ items: [] });
      }),
    );
    renderRoute(address);
    const timeline = await screen.findByRole('list', { name: 'Steps of the run' });
    await userEvent.click(await within(timeline).findByRole('button', { name: /Greet the customer/ }));
    await userEvent.click(await screen.findByRole('tab', { name: /^Output/ }));
    expect(await screen.findByText('You cannot see the data of this run. Ask an administrator of the project for access to run data.')).toBeVisible();
    expect(asked).toBe(false);
  });

  it('forbidden items: the steps stay, the items say they are not allowed', async () => {
    server.use(
      ...beforeRun(),
      executionApi.run(() => HttpResponse.json(run)),
      executionApi.nodes([greetingStep]),
      executionApi.version(),
      http.get(`/api/v1/executions/${runId}/data`, () => problem(403, 'forbidden', 'forbidden')),
    );
    renderRoute(address);
    const timeline = await screen.findByRole('list', { name: 'Steps of the run' });
    await userEvent.click(await within(timeline).findByRole('button', { name: /Greet the customer/ }));
    await userEvent.click(await screen.findByRole('tab', { name: /^Input/ }));
    expect(await screen.findByText('You cannot see the data of this run. Ask an administrator of the project for access to run data.')).toBeVisible();
  });

  it('uncertain shows the resolution panel and never retry as primary', async () => {
    server.use(
      ...beforeRun(),
      executionApi.run(() => HttpResponse.json(reviewRun), reviewRunId),
      executionApi.nodes([{ ...greetingStep, executionId: reviewRunId }], reviewRunId),
      executionApi.version(),
      executionApi.data([], reviewRunId),
    );
    renderRoute(`/orgs/acme/executions/${reviewRunId}`);
    expect(await screen.findByRole('heading', { name: 'Review before continuing' })).toBeVisible();
    // The status is the words of the review state, never a success and never a failure.
    const header = screen.getByRole('heading', { level: 2, name: /Order notifications/ }).closest('header');
    expect(header).toHaveTextContent('Needs review');
    expect(header).not.toHaveTextContent('Succeeded');
    expect(header).not.toHaveTextContent('Failed');
    // A retry is not offered, and the choices wait for the server instead of pretending to work.
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Check again whether it was applied' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'It was not applied: send it again' })).toBeDisabled();
    expect(screen.getByText(/Resolving the operation from this page is not available yet/)).toBeVisible();
  });

  it('failed offers retry with the confirmation text of Q10', async () => {
    const created = { ...run, id: '0195f1a0-0000-7000-8000-0000000000f9', mode: 'RETRY', status: 'QUEUED', retryOfExecutionId: runId, failure: undefined };
    let retried = false;
    server.use(
      ...beforeRun(),
      ...failedRun(),
      executionApi.retry(() => {
        retried = true;
        return HttpResponse.json(created, { status: 201 });
      }),
      executionApi.run(() => HttpResponse.json(created), created.id),
      executionApi.nodes([], created.id),
    );
    const { router } = renderRoute(address);
    // The button says "Retry": it will say "Retry from the failure" only when the server proves that it repeats just what failed.
    await userEvent.click(await screen.findByRole('button', { name: 'Retry' }));
    const dialog = await screen.findByRole('dialog', { name: 'Retry this run?' });
    expect(dialog).toHaveTextContent('Orcon starts a new run of the same version of the flow (2) that repeats this failed run.');
    expect(dialog).toHaveTextContent('This run stays in the history and the new one is marked as its retry.');
    expect(dialog).toHaveTextContent('Orcon has not yet confirmed that only the failed items are repeated');
    expect(retried).toBe(false);
    await userEvent.click(within(dialog).getByRole('button', { name: 'Retry run' }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/orgs/acme/executions/${created.id}`);
    });
    expect(retried).toBe(true);
  });

  it('retry: a server that cannot retry yet is told calmly and nothing changes', async () => {
    server.use(...beforeRun(), ...failedRun(), executionApi.retry(() => problem(503, 'Service unavailable', 'service-unavailable')));
    const { router } = renderRoute(address);
    await userEvent.click(await screen.findByRole('button', { name: 'Retry' }));
    const dialog = await screen.findByRole('dialog', { name: 'Retry this run?' });
    await userEvent.click(within(dialog).getByRole('button', { name: 'Retry run' }));
    expect(await within(dialog).findByRole('heading', { name: 'Retrying is not available yet' })).toBeVisible();
    expect(within(dialog).queryByText('Service unavailable')).toBeNull();
    expect(router.state.location.pathname).toBe(address);
  });

  it('a run that succeeded has no retry', async () => {
    server.use(...beforeRun(), executionApi.run(() => HttpResponse.json(otherRun), otherRun.id), executionApi.nodes([], otherRun.id), executionApi.version(), executionApi.data([], otherRun.id));
    renderRoute(`/orgs/acme/executions/${otherRun.id}`);
    expect(await screen.findByRole('heading', { level: 2, name: /Order notifications/ })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull();
  });

  it('the project page of a run fills the page and goes back to the runs of the project', async () => {
    server.use(...beforeRun(), ...failedRun());
    renderRoute(`/orgs/acme/projects/ventas/executions/${runId}`);
    expect(await screen.findByRole('heading', { level: 1, name: /Order notifications/ })).toBeVisible();
    expect(screen.getByRole('link', { name: 'Back to executions' })).toHaveAttribute('href', '/orgs/acme/projects/ventas/executions');
  });
});
