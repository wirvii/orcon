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

describe('the shell', () => {
  it('lists the sections and marks the one being shown', async () => {
    renderRoute('/');
    expect(await screen.findByRole('heading', { level: 1, name: 'Welcome to Orcon' })).toBeVisible();
    expect(sectionLinks('Start').length).toBeGreaterThan(0);
    expect(sectionLinks('Settings').length).toBeGreaterThan(0);
    for (const link of sectionLinks('Start')) expect(link).toHaveAttribute('aria-current', 'page');
    for (const link of sectionLinks('Settings')) expect(link).not.toHaveAttribute('aria-current');
    expect(sectionLinks('Settings')[0]).toHaveAttribute('href', '/settings/appearance');
  });

  it('goes to another section without reloading the page', async () => {
    const { router } = renderRoute('/');
    await screen.findByRole('heading', { level: 1, name: 'Welcome to Orcon' });
    const marker = document.createElement('meta');
    document.head.append(marker);
    const settings = sectionLinks('Settings')[0];
    if (!settings) throw new Error('The shell has no Settings link.');
    await userEvent.click(settings);

    expect(await screen.findByRole('heading', { level: 1, name: 'Appearance and language' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/settings/appearance');
    expect(marker.isConnected).toBe(true);
    for (const link of sectionLinks('Settings')) expect(link).toHaveAttribute('aria-current', 'page');
    marker.remove();
  });

  it('opens the flow editor from the navigation, at its own address, and marks it as the section being shown', async () => {
    const { router } = renderRoute('/');
    await screen.findByRole('heading', { level: 1, name: 'Welcome to Orcon' });
    const editor = sectionLinks('Flow editor')[0];
    if (!editor) throw new Error('The shell has no Flow editor link.');
    expect(editor).toHaveAttribute('href', '/flows/01930000-0000-7000-8000-00000000ed5a');
    await userEvent.click(editor);

    expect(await screen.findByRole('heading', { level: 1, name: 'Order notifications' })).toBeVisible();
    expect(router.state.location.pathname).toBe('/flows/01930000-0000-7000-8000-00000000ed5a');
    for (const link of sectionLinks('Flow editor')) expect(link).toHaveAttribute('aria-current', 'page');
    for (const link of sectionLinks('Start')) expect(link).not.toHaveAttribute('aria-current');
  });

  it('says a page that is not a flow id does not exist', async () => {
    renderRoute('/flows/not-a-flow');
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeVisible();
  });

  it('leaves a click with a modifier key to the browser', async () => {
    const { router } = renderRoute('/');
    await screen.findByRole('heading', { level: 1, name: 'Welcome to Orcon' });
    const settings = sectionLinks('Settings')[0];
    if (!settings) throw new Error('The shell has no Settings link.');
    // The browser would open a new tab; the router must not navigate.
    settings.addEventListener('click', (event) => {
      event.preventDefault();
    });
    await userEvent.keyboard('{Control>}');
    await userEvent.click(settings);
    await userEvent.keyboard('{/Control}');
    expect(router.state.location.pathname).toBe('/');
  });

  it('marks the first useful screen of the start page', async () => {
    const mark = vi.spyOn(performance, 'mark');
    renderRoute('/');
    await screen.findByRole('heading', { level: 1, name: 'Welcome to Orcon' });
    expect(mark).toHaveBeenCalledWith(FIRST_USEFUL_MARK, { detail: { route: '/' } });
    mark.mockRestore();
  });

  it('shows the not-found page for an address that is no route, inside the shell', async () => {
    renderRoute('/no/such/page');
    expect(await screen.findByRole('heading', { level: 1, name: 'Page not found' })).toBeVisible();
    expect(screen.getByRole('link', { name: 'Go to the start page' })).toHaveAttribute('href', '/');
    expect(sectionLinks('Settings').length).toBeGreaterThan(0);
  });

  it('says when the browser is offline and stops saying it when the connection returns', async () => {
    renderRoute('/');
    await screen.findByRole('heading', { level: 1, name: 'Welcome to Orcon' });
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
    renderRoute('/', { source });
    await screen.findByRole('heading', { level: 1, name: 'Welcome to Orcon' });
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
