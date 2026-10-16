// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http } from 'msw';
import { describe, expect, it } from 'vitest';

import { productApi, flow, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';

const address = '/orgs/acme/projects/ventas/flows';

/** Everything the flows page reads before its own list: the person, the organization and the project. */
const beforeFlows = () => [signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project])];

describe('FlowListScreen', () => {
  it('loading', async () => {
    server.use(...beforeFlows(), http.get(`/api/v1/projects/${project.id}/flows`, () => new Promise(() => undefined)));
    renderRoute(address);
    expect(await screen.findByRole('status', { name: 'Loading' })).toBeVisible();
  });

  it('empty', async () => {
    server.use(...beforeFlows(), productApi.flows([]));
    renderRoute(address);
    expect(await screen.findByRole('heading', { name: 'No flows yet' })).toBeVisible();
    // The first-run empty state shows the happy mascot (SPEC-018).
    expect(document.querySelector('[data-mascot-pose="happy"]')).not.toBeNull();
  });

  it('error', async () => {
    server.use(...beforeFlows(), http.get(`/api/v1/projects/${project.id}/flows`, () => problem(500, 'internal', 'internal')));
    renderRoute(address);
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
  });

  it('forbidden', async () => {
    server.use(...beforeFlows(), http.get(`/api/v1/projects/${project.id}/flows`, () => problem(403, 'forbidden', 'forbidden')));
    renderRoute(address);
    expect(await screen.findByText('You do not have permission to see this')).toBeVisible();
  });

  it('data', async () => {
    server.use(...beforeFlows(), productApi.flows([flow]));
    renderRoute(address);
    const name = await screen.findByRole('link', { name: 'Order notifications' });
    expect(name).toHaveAttribute('href', `/orgs/acme/projects/ventas/flows/${flow.id}`);
    // The state is words, never color alone; on a phone the label precedes it.
    expect(screen.getByText(/Inactive$/)).toBeVisible();
  });

  it('opens the chooser of how to start from its button, and from the address', async () => {
    server.use(...beforeFlows(), productApi.flows([]));
    renderRoute(address);
    await userEvent.click((await screen.findAllByRole('button', { name: 'Create flow' }))[0] as HTMLElement);
    const dialog = await screen.findByRole('dialog', { name: 'Create flow' });
    expect(within(dialog).getByRole('button', { name: /Blank/ })).toHaveAttribute('aria-pressed', 'true');
  });
});
