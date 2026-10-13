// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
//
// Template for features/<area>/screens/<Name>Screen.test.tsx (testing.md TE-1
// to TE-6). Every screen covers its five states against Mock Service Worker
// handlers generated from the API contract; unhandled requests fail the test.
// Query by role and accessible name; no DOM snapshots.
// `renderRoute` and `server` stand for the helpers of the `screens` Vitest
// project of apps/web; adjust the import to where that setup lives.

import { screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { getWidgetHandler, getWidgetResponse } from '@orcon/api-contract/msw';
import { renderRoute, server } from '../../../test/screen-harness';

const path = '/widgets/0b9f6b1e-8f0e-4a8e-9a39-6f0a3c1d2e4f';

describe('WidgetScreen', () => {
  it('shows a skeleton while loading', async () => {
    server.use(getWidgetHandler(() => new Promise(() => undefined)));
    renderRoute(path);
    expect(await screen.findByRole('status', { name: /loading/i })).toBeVisible();
  });

  it('shows the empty state', async () => {
    server.use(getWidgetHandler(() => HttpResponse.json({ ...getWidgetResponse(), items: [] })));
    renderRoute(path);
    expect(await screen.findByRole('heading', { name: /no widgets yet/i })).toBeVisible();
  });

  it('shows the error with a retry action', async () => {
    server.use(
      http.get('*/api/v1/widgets/:id', () =>
        HttpResponse.json({ type: 'about:blank', title: 'Internal error', status: 500 }, { status: 500 }),
      ),
    );
    renderRoute(path);
    expect(await screen.findByRole('button', { name: /retry/i })).toBeEnabled();
  });

  it('shows the permission message without retry', async () => {
    server.use(
      http.get('*/api/v1/widgets/:id', () =>
        HttpResponse.json({ type: 'about:blank', title: 'Forbidden', status: 403 }, { status: 403 }),
      ),
    );
    renderRoute(path);
    expect(await screen.findByText(/you do not have permission/i)).toBeVisible();
    expect(screen.queryByRole('button', { name: /retry/i })).toBeNull();
  });

  it('shows the data', async () => {
    const widget = getWidgetResponse({ name: 'Nightly sync' });
    server.use(getWidgetHandler(() => HttpResponse.json(widget)));
    renderRoute(path);
    expect(await screen.findByRole('heading', { name: 'Nightly sync' })).toBeVisible();
  });
});
