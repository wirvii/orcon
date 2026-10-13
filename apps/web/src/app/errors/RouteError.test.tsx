// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Providers } from '@orcon/ui';
import { createMemoryHistory, createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { appMessages } from '../../i18n/messages';

import { ContractError } from './contract-error';
import { RouteError } from './RouteError';

afterEach(() => {
  vi.restoreAllMocks();
});

/** Shows the route error component for an error, inside a router, as the router would. */
function renderError(error: unknown, reset = vi.fn()) {
  const root = createRootRoute({ component: () => <RouteError error={error as Error} reset={reset} /> });
  const router = createRouter({ routeTree: root, history: createMemoryHistory({ initialEntries: ['/'] }) });
  render(
    <Providers messages={appMessages}>
      <RouterProvider router={router} />
    </Providers>,
  );
  return { reset, router };
}

describe('RouteError', () => {
  it('shows the problem with a retry that tries again', async () => {
    const { reset, router } = renderError({ status: 500, title: 'Internal error', requestId: 'req_7' });
    expect(await screen.findByText('Internal error')).toBeVisible();
    const invalidate = vi.spyOn(router, 'invalidate');
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(reset).toHaveBeenCalledTimes(1);
    expect(invalidate).toHaveBeenCalledTimes(1);
  });

  it('says what happened for a permission error and offers no retry', async () => {
    renderError({ status: 403, title: 'Forbidden' });
    expect(await screen.findByRole('heading', { name: 'You do not have permission to see this' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
  });

  it('shows the not-found page for a missing resource', async () => {
    renderError({ status: 404, title: 'Not found' });
    expect(await screen.findByRole('heading', { level: 1, name: 'Page not found' })).toBeVisible();
  });

  it('shows the generic message for an error that is not a problem', async () => {
    renderError(new TypeError('Failed to fetch'));
    expect(await screen.findByText('Something went wrong.')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeEnabled();
  });

  it('shows a contract error as the generic message and logs where it came from, not what it held', async () => {
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const broken = new ContractError('response of GET /flows', z.string().safeParse('x'.length).error?.issues ?? []);
    renderError(broken);
    expect(await screen.findByText('Something went wrong.')).toBeVisible();
    expect(log).toHaveBeenCalledTimes(1);
    expect(String(log.mock.calls[0]?.[0])).toBe('Contract error: response of GET /flows');
  });
});
