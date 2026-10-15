// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { executionApi, greetingData, greetingStep, run, runId, slackStep } from '../../../../test/execution-fixtures';
import { productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';

const address = `/orgs/acme/projects/ventas/executions/${runId}`;

/** Everything the run page reads before its own data: the person, the organization and the project. */
const beforeRun = () => [signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project])];

describe('ExecutionDetailScreen', () => {
  it('loading', async () => {
    server.use(...beforeRun(), http.get(`/api/v1/executions/${runId}`, () => new Promise(() => undefined)));
    renderRoute(address);
    expect(await screen.findByRole('status', { name: 'Loading' })).toBeVisible();
  });

  it('error', async () => {
    server.use(...beforeRun(), executionApi.run(() => problem(500, 'internal', 'internal')), executionApi.nodes([]));
    renderRoute(address);
    expect(await screen.findByRole('alert')).toBeVisible();
  });

  it('data: the status, why it failed, each step and the items it received and produced', async () => {
    server.use(
      ...beforeRun(),
      executionApi.run(() => HttpResponse.json(run)),
      executionApi.nodes([greetingStep, slackStep]),
      executionApi.data([greetingData]),
    );
    renderRoute(address);
    expect(await screen.findByRole('heading', { level: 1, name: 'Run 0195f1a0' })).toBeVisible();
    // The run and its failing step both show a Failed badge and the same failure text.
    expect(screen.getAllByText('Failed').length).toBeGreaterThan(1);
    expect(screen.getAllByText('The webhook answered 500.').length).toBeGreaterThan(0);
    expect(screen.getByRole('heading', { level: 3, name: 'greeting' })).toBeVisible();
    expect(screen.getByRole('heading', { level: 3, name: 'slack' })).toBeVisible();
    expect(screen.getByText('Attempts: 3')).toBeVisible();
    // The items arrive after the steps: the region is found by waiting for it, not at once.
    expect(await screen.findByRole('region', { name: 'Output of greeting, port main' })).toBeVisible();
    expect(screen.getByText(/Hello, Ana/)).toBeVisible();
  });

  it('hidden: the items are not asked for and the page says why', async () => {
    let asked = false;
    server.use(
      ...beforeRun(),
      executionApi.run(() => HttpResponse.json({ ...run, dataHidden: true })),
      executionApi.nodes([greetingStep]),
      http.get(`/api/v1/executions/${runId}/data`, () => {
        asked = true;
        return HttpResponse.json({ items: [] });
      }),
    );
    renderRoute(address);
    // The banner and each step say the same thing.
    expect((await screen.findAllByText('You cannot see the data of this run. Ask an administrator of the project for access to run data.')).length).toBeGreaterThan(1);
    expect(asked).toBe(false);
  });

  it('forbidden items: the steps stay, the items say they are not allowed', async () => {
    server.use(
      ...beforeRun(),
      executionApi.run(() => HttpResponse.json(run)),
      executionApi.nodes([greetingStep]),
      http.get(`/api/v1/executions/${runId}/data`, () => problem(403, 'forbidden', 'forbidden')),
    );
    renderRoute(address);
    expect(await screen.findByRole('heading', { level: 3, name: 'greeting' })).toBeVisible();
    expect(await screen.findByText('You cannot see the data of this run. Ask an administrator of the project for access to run data.')).toBeVisible();
  });
});
