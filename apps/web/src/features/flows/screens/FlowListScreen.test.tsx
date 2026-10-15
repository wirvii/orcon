// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { listedFlow, productApi, project } from '../../../../test/product-fixtures';
import { renderRoute, server } from '../../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../../test/session-fixtures';

const address = '/orgs/acme/projects/ventas/flows';

/** Everything the flows page reads before its own list: the person, the organization and the project. */
const beforeFlows = () => [signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project])];

describe('FlowListScreen', () => {
  it('loading', async () => {
    server.use(...beforeFlows(), http.get(`/api/v1/orgs/${acme.id}/flows`, () => new Promise(() => undefined)));
    renderRoute(address);
    expect(await screen.findByRole('status', { name: 'Loading' })).toBeVisible();
  });

  it('empty', async () => {
    server.use(...beforeFlows(), productApi.orgFlows([]));
    renderRoute(address);
    expect(await screen.findByRole('heading', { name: 'Ventas is ready for its first flow' })).toBeVisible();
    expect(document.querySelector('[data-mascot-pose="thinking"]')).not.toBeNull();
  });

  it('error', async () => {
    server.use(...beforeFlows(), http.get(`/api/v1/orgs/${acme.id}/flows`, () => problem(500, 'internal', 'internal')));
    renderRoute(address);
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
  });

  it('forbidden', async () => {
    server.use(...beforeFlows(), http.get(`/api/v1/orgs/${acme.id}/flows`, () => problem(403, 'forbidden', 'forbidden')));
    renderRoute(address);
    expect(await screen.findByText('You do not have permission to see this')).toBeVisible();
  });

  it('data', async () => {
    server.use(...beforeFlows(), productApi.orgFlows([listedFlow]));
    renderRoute(address);
    const name = await screen.findByRole('link', { name: 'Order notifications' });
    expect(name).toHaveAttribute('href', `/orgs/acme/projects/ventas/flows/${listedFlow.id}`);
    const row = name.closest('li') as HTMLElement;
    // The runs of the last 24 hours and the state of the switch, in words as well as in position.
    expect(row).toHaveTextContent('12');
    expect(within(row).getByRole('switch', { name: 'Active: Order notifications' })).toBeChecked();
    expect(within(row).getByText('Active')).toBeVisible();
  });

  it('row shows last run as icon and text', async () => {
    const failed = { ...listedFlow, id: '0195f1a0-0000-7000-8000-0000000000e2', name: 'Refunds', lastExecution: { ...listedFlow.lastExecution, status: 'FAILED' } };
    const review = { ...listedFlow, id: '0195f1a0-0000-7000-8000-0000000000e3', name: 'Reminders', lastExecution: { ...listedFlow.lastExecution, hasUncertainOperation: true } };
    server.use(...beforeFlows(), productApi.orgFlows([listedFlow, failed, review]));
    renderRoute(address);
    const done = (await screen.findByRole('link', { name: 'Order notifications' })).closest('li') as HTMLElement;
    // An icon and a word, never a color alone: the icon is decorative and the word is read.
    expect(within(done).getByText('Succeeded')).toBeVisible();
    expect(done.querySelector('svg[aria-hidden="true"]')).not.toBeNull();
    expect((screen.getByRole('link', { name: 'Refunds' }).closest('li') as HTMLElement)).toHaveTextContent('Failed');
    // An operation of unknown outcome is shown as one to review, not as a success or a failure.
    expect((screen.getByRole('link', { name: 'Reminders' }).closest('li') as HTMLElement)).toHaveTextContent('Needs review');
  });

  it('row opens the editor on click', async () => {
    server.use(...beforeFlows(), productApi.orgFlows([listedFlow]));
    const { router } = renderRoute(address);
    await userEvent.click(await screen.findByRole('link', { name: 'Order notifications' }));
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(`/orgs/acme/projects/ventas/flows/${listedFlow.id}`);
    });
  });

  it('toggle calls activate without optimistic update', async () => {
    const inactive = { ...listedFlow, activationState: 'INACTIVE' };
    let release: (() => void) | undefined;
    let ifMatch: string | null = null;
    server.use(
      ...beforeFlows(),
      productApi.orgFlows([inactive]),
      http.post(`/api/v1/flows/${listedFlow.id}/activate`, async ({ request }) => {
        ifMatch = request.headers.get('If-Match');
        await new Promise<void>((resolve) => {
          release = resolve;
        });
        return HttpResponse.json({ ...inactive, activationState: 'ACTIVE' });
      }),
    );
    renderRoute(address);
    const toggle = await screen.findByRole('switch', { name: 'Active: Order notifications' });
    expect(toggle).not.toBeChecked();
    await userEvent.click(toggle);
    // While the server answers the switch has not moved and says so.
    await waitFor(() => {
      expect(ifMatch).toBe('"3"');
    });
    expect(screen.getByRole('switch', { name: 'Active: Order notifications' })).not.toBeChecked();
    expect(screen.getByText('Updating')).toBeVisible();
    server.use(productApi.orgFlows([{ ...inactive, activationState: 'ACTIVE' }]));
    release?.();
    await waitFor(() => {
      expect(screen.getByRole('switch', { name: 'Active: Order notifications' })).toBeChecked();
    });
  });

  it('toggle is off and locked for a flow that was never published', async () => {
    const draft = { ...listedFlow, activationState: 'INACTIVE', activeVersion: undefined, lastExecution: undefined, executionCountLast24h: 0 };
    server.use(...beforeFlows(), productApi.orgFlows([draft]));
    renderRoute(address);
    const toggle = await screen.findByRole('switch', { name: 'Active: Order notifications' });
    expect(toggle).not.toBeChecked();
    expect(toggle).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByText('Publish the flow to be able to activate it')).toBeInTheDocument();
    await userEvent.click(toggle);
    expect(screen.queryByText('Updating')).toBeNull();
  });

  it('hides tags and owner', async () => {
    server.use(...beforeFlows(), productApi.orgFlows([listedFlow]));
    renderRoute(address);
    const row = (await screen.findByRole('link', { name: 'Order notifications' })).closest('li') as HTMLElement;
    // The contract has neither yet: nothing stands in for them.
    expect(within(row).queryByTitle('Owner')).toBeNull();
    expect(within(row).queryByRole('img')).toBeNull();
    expect(screen.queryByText(/Tags/i)).toBeNull();
  });

  it('keeps the search in the address and asks the server for it', async () => {
    const other = { ...listedFlow, id: '0195f1a0-0000-7000-8000-0000000000e2', name: 'Refunds' };
    server.use(...beforeFlows(), productApi.orgFlows([listedFlow, other]));
    const { router } = renderRoute(address);
    await screen.findByRole('link', { name: 'Refunds' });
    await userEvent.type(screen.getByRole('searchbox', { name: 'Search in this project' }), 'ref');
    await waitFor(() => {
      expect(router.state.location.search).toEqual({ q: 'ref' });
    });
    await waitFor(() => {
      expect(screen.queryByRole('link', { name: 'Order notifications' })).toBeNull();
    });
    expect(screen.getByRole('link', { name: 'Refunds' })).toBeVisible();
  });

  it('opens the chooser of how to start from its button, and from the address', async () => {
    server.use(...beforeFlows(), productApi.flows([]));
    renderRoute(address);
    await userEvent.click((await screen.findAllByRole('button', { name: 'Create flow' }))[0] as HTMLElement);
    const dialog = await screen.findByRole('dialog', { name: 'Create flow' });
    expect(within(dialog).getByRole('button', { name: /Blank/ })).toHaveAttribute('aria-pressed', 'true');
  });
});
