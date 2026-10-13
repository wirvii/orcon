// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { z } from 'zod';

import { renderInApp, server } from '../../../test/screen-harness';
import { parseWith } from '../boundary/parse';

import { QueryState } from './QueryState';

const widgetsSchema = z.array(z.object({ id: z.string(), name: z.string() }));

/** A screen that loads widgets the way a feature does: one query, rendered through QueryState. */
function Widgets() {
  const query = useQuery({
    queryKey: ['org', 'widgets'],
    queryFn: async () => {
      const response = await fetch('/api/v1/widgets');
      const body: unknown = await response.json();
      if (!response.ok) throw body;
      return parseWith(widgetsSchema, body, 'response of GET /widgets');
    },
  });
  return (
    <QueryState query={query} empty={{ title: 'No widgets yet', description: 'Create the first one.' }}>
      {(widgets) => (
        <ul>
          {widgets.map((widget) => (
            <li key={widget.id}>{widget.name}</li>
          ))}
        </ul>
      )}
    </QueryState>
  );
}

const problem = (status: number, title: string) => HttpResponse.json({ type: 'about:blank', title, status }, { status, headers: { 'content-type': 'application/problem+json' } });

describe('QueryState', () => {
  it('shows placeholders while loading', async () => {
    server.use(http.get('*/api/v1/widgets', () => new Promise(() => undefined)));
    renderInApp(<Widgets />);
    const region = await screen.findByRole('status', { name: 'Loading' });
    expect(region).toHaveAttribute('aria-busy', 'true');
  });

  it('shows the empty state with its next step', async () => {
    server.use(http.get('*/api/v1/widgets', () => HttpResponse.json([])));
    renderInApp(<Widgets />);
    expect(await screen.findByRole('heading', { name: 'No widgets yet' })).toBeVisible();
    expect(screen.getByText('Create the first one.')).toBeVisible();
  });

  it('shows the error with a retry that loads the data again', async () => {
    let attempts = 0;
    server.use(
      http.get('*/api/v1/widgets', () => {
        attempts += 1;
        return attempts === 1 ? problem(500, 'Internal error') : HttpResponse.json([{ id: '1', name: 'Nightly sync' }]);
      }),
    );
    renderInApp(<Widgets />);
    expect(await screen.findByText('Internal error')).toBeVisible();
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByText('Nightly sync')).toBeVisible();
    expect(attempts).toBe(2);
  });

  it('shows the permission message and offers no retry', async () => {
    server.use(http.get('*/api/v1/widgets', () => problem(403, 'Forbidden')));
    renderInApp(<Widgets />);
    expect(await screen.findByRole('heading', { name: 'You do not have permission to see this' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
  });

  it('shows the data', async () => {
    server.use(http.get('*/api/v1/widgets', () => HttpResponse.json([{ id: '1', name: 'Nightly sync' }, { id: '2', name: 'Weekly report' }])));
    renderInApp(<Widgets />);
    expect(await screen.findByText('Nightly sync')).toBeVisible();
    expect(screen.getByText('Weekly report')).toBeVisible();
    expect(screen.queryByRole('status', { name: 'Loading' })).toBeNull();
  });

  it('shows a response that breaks its schema as a generic error, without the content', async () => {
    server.use(http.get('*/api/v1/widgets', () => HttpResponse.json([{ id: 1, name: 'api-key-123' }])));
    renderInApp(<Widgets />);
    expect(await screen.findByText('Something went wrong.')).toBeVisible();
    expect(screen.queryByText(/api-key-123/)).toBeNull();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeEnabled();
  });

  it('shows the answer in Spanish when the locale is Spanish', async () => {
    window.localStorage.setItem('orcon.locale', 'es');
    server.use(http.get('*/api/v1/widgets', () => problem(403, 'Forbidden')));
    renderInApp(<Widgets />);
    expect(await screen.findByRole('heading', { name: 'No tienes permiso para ver esto' })).toBeVisible();
    await waitFor(() => {
      expect(document.documentElement).toHaveAttribute('lang', 'es');
    });
  });
});
