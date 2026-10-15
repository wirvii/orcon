// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { onlineManager } from '@tanstack/react-query';
import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { renderRoute } from '../../../test/screen-harness';
import type { LiveConnectionState, LiveEventSource } from '../live-events/dispatcher';
import { FIRST_USEFUL_MARK } from '../perf/marks';

afterEach(() => {
  onlineManager.setOnline(true);
});

/** The links the shell shows for a section: a sidebar, a bottom bar and a drawer each carry one. */
const sectionLinks = (name: string) => screen.getAllByRole('link', { name, hidden: true });

describe('the page frame', () => {
  it('lists the settings and the API reference, and marks the section being shown', async () => {
    renderRoute('/settings/appearance');
    expect(await screen.findByRole('heading', { level: 1, name: 'Appearance and language' })).toBeVisible();
    for (const link of sectionLinks('Settings')) expect(link).toHaveAttribute('aria-current', 'page');
    expect(sectionLinks('API reference')[0]).toHaveAttribute('href', '/api/reference/');
    expect(sectionLinks('API reference')[0]).not.toHaveAttribute('target', '_blank');
  });

  it('leaves a click with a modifier key to the browser', async () => {
    const { router } = renderRoute('/settings/appearance');
    await screen.findByRole('heading', { level: 1, name: 'Appearance and language' });
    const settings = sectionLinks('Settings')[0];
    if (!settings) throw new Error('The shell has no Settings link.');
    // The browser would open a new tab; the router must not navigate.
    settings.addEventListener('click', (event) => {
      event.preventDefault();
    });
    await userEvent.keyboard('{Control>}');
    await userEvent.click(settings);
    await userEvent.keyboard('{/Control}');
    expect(router.state.location.pathname).toBe('/settings/appearance');
  });

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
    renderRoute('/settings/appearance', { source });
    await screen.findByRole('heading', { level: 1, name: 'Appearance and language' });
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
