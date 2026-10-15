// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { onlineManager } from '@tanstack/react-query';
import { act, screen, waitFor } from '@testing-library/react';
import { http } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { flow, productApi, project } from '../../../test/product-fixtures';
import { renderRoute, server } from '../../../test/screen-harness';
import { acme, problem, signedIn } from '../../../test/session-fixtures';
import type { LiveConnectionState, LiveEventSource } from '../live-events/dispatcher';
import { FIRST_USEFUL_MARK } from '../perf/marks';

beforeEach(() => {
  // Nobody is signed in, unless a test says so.
  server.use(http.get('/api/v1/users/me', () => problem(401, 'unauthenticated', 'unauthenticated')));
});

afterEach(() => {
  onlineManager.setOnline(true);
});

describe('the page frame', () => {
  it('shows the not-found page for an address that is no route', async () => {
    renderRoute('/no/such/page');
    expect(await screen.findByRole('heading', { level: 1, name: 'Page not found' })).toBeVisible();
    expect(screen.getByRole('link', { name: 'Go to the start page' })).toHaveAttribute('href', '/');
  });

  it('marks the first useful screen of the sign-in page', async () => {
    const mark = vi.spyOn(performance, 'mark');
    renderRoute('/login');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    expect(mark).toHaveBeenCalledWith(FIRST_USEFUL_MARK, { detail: { route: '/login' } });
    mark.mockRestore();
  });

  it('says when the browser is offline and stops saying it when the connection returns', async () => {
    renderRoute('/login');
    await screen.findByRole('heading', { level: 1, name: 'Sign in' });
    expect(screen.queryByText('You are offline')).toBeNull();
    act(() => {
      onlineManager.setOnline(false);
    });
    expect(await screen.findByText('You are offline')).toBeVisible();
    act(() => {
      onlineManager.setOnline(true);
    });
    await waitFor(() => {
      expect(screen.queryByText('You are offline')).toBeNull();
    });
  });

  it('says when live updates are paused', async () => {
    let report: ((state: LiveConnectionState) => void) | undefined;
    const source: LiveEventSource = {
      subscribe: (_onEvent, onState) => {
        report = onState;
        return () => undefined;
      },
    };
    server.use(signedIn.currentUser, signedIn.organizations([acme]), productApi.projects([project]), productApi.flows([flow]));
    renderRoute('/orgs/acme/overview', { source });
    await screen.findByRole('heading', { level: 1, name: /^Good (morning|afternoon|evening), Ana$/ });
    expect(screen.queryByText('Live updates are paused')).toBeNull();
    act(() => {
      report?.('paused');
    });
    expect(await screen.findByText('Live updates are paused')).toBeVisible();
    act(() => {
      report?.('live');
    });
    await waitFor(() => {
      expect(screen.queryByText('Live updates are paused')).toBeNull();
    });
  });
});
