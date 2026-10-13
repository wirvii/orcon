// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { renderRoute } from '../../../../test/screen-harness';
import { FIRST_USEFUL_MARK } from '../../../app/perf/marks';

const heading = (name: string) => screen.findByRole('heading', { level: 2, name });

describe('the appearance screen', () => {
  it('shows the page, the links to its sections and the form', async () => {
    renderRoute('/settings/appearance');
    expect(await screen.findByRole('heading', { level: 1, name: 'Appearance and language' })).toBeVisible();
    const sections = screen.getByRole('navigation', { name: 'On this page' });
    expect(within(sections).getAllByRole('link').map((link) => link.textContent)).toEqual(['Theme', 'Density', 'Language']);
    expect(screen.getByRole('radiogroup', { name: 'Color theme' })).toBeVisible();
    expect(screen.getByRole('radiogroup', { name: 'Row density' })).toBeVisible();
    expect(screen.getByRole('combobox', { name: 'Language' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Save preferences' })).toBeEnabled();
  });

  it('moves focus to the section named in the address', async () => {
    renderRoute('/settings/appearance?section=language');
    const language = await heading('Language');
    await waitFor(() => {
      expect(language).toHaveFocus();
    });
    expect(within(screen.getByRole('navigation', { name: 'On this page' })).getByRole('link', { name: 'Language' })).toHaveAttribute('aria-current', 'page');
  });

  it('moves focus nowhere when the address names no section, or one that does not exist', async () => {
    const { unmount } = renderRoute('/settings/appearance');
    await heading('Theme');
    expect(document.body).toHaveFocus();
    unmount();
    renderRoute('/settings/appearance?section=colors');
    await heading('Theme');
    expect(document.body).toHaveFocus();
    expect(screen.getByRole('heading', { level: 1, name: 'Appearance and language' })).toBeVisible();
  });

  it('follows a link to another section: the address and the focus change, the page does not reload', async () => {
    const { router } = renderRoute('/settings/appearance?section=theme');
    await heading('Theme');
    await userEvent.click(within(screen.getByRole('navigation', { name: 'On this page' })).getByRole('link', { name: 'Density' }));
    await waitFor(() => {
      expect(router.state.location.search).toEqual({ section: 'density' });
    });
    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2, name: 'Density' })).toHaveFocus();
    });
  });

  it('applies and saves the changed choices', async () => {
    renderRoute('/settings/appearance');
    await heading('Theme');
    await userEvent.click(screen.getByRole('radio', { name: 'Dark' }));
    await userEvent.click(screen.getByRole('radio', { name: 'Compact' }));
    await userEvent.click(screen.getByRole('button', { name: 'Save preferences' }));

    expect(await screen.findByRole('status')).toHaveTextContent('Preferences saved');
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark');
    expect(document.documentElement).toHaveAttribute('data-density', 'compact');
    expect(window.localStorage.getItem('orcon.theme')).toBe('dark');
    expect(window.localStorage.getItem('orcon.density')).toBe('compact');
    expect(screen.getByRole('radio', { name: 'Dark' })).toBeChecked();
  });

  it('changes the language of the whole page', async () => {
    renderRoute('/settings/appearance');
    await heading('Language');
    await userEvent.click(screen.getByRole('combobox', { name: 'Language' }));
    await userEvent.click(await screen.findByRole('option', { name: 'Español' }));
    await userEvent.click(screen.getByRole('button', { name: 'Save preferences' }));

    expect(await screen.findByRole('heading', { level: 1, name: 'Apariencia e idioma' })).toBeVisible();
    expect(await screen.findByRole('status')).toHaveTextContent('Preferencias guardadas');
    expect(document.documentElement).toHaveAttribute('lang', 'es');
    expect(window.localStorage.getItem('orcon.locale')).toBe('es');
  });

  it('keeps what was not changed, and writes nothing when nothing changed', async () => {
    renderRoute('/settings/appearance');
    await heading('Theme');
    await userEvent.click(screen.getByRole('button', { name: 'Save preferences' }));
    expect(await screen.findByRole('status')).toHaveTextContent('Preferences saved');
    expect(window.localStorage.getItem('orcon.theme')).toBeNull();
    expect(window.localStorage.getItem('orcon.density')).toBeNull();
    expect(window.localStorage.getItem('orcon.locale')).toBeNull();
  });

  it('hides the confirmation as soon as another change is made', async () => {
    renderRoute('/settings/appearance');
    await heading('Theme');
    await userEvent.click(screen.getByRole('radio', { name: 'Light' }));
    await userEvent.click(screen.getByRole('button', { name: 'Save preferences' }));
    expect(await screen.findByRole('status')).toBeVisible();
    await userEvent.click(screen.getByRole('radio', { name: 'Dark' }));
    await waitFor(() => {
      expect(screen.queryByRole('status')).toBeNull();
    });
  });

  it('starts from what is already saved', async () => {
    window.localStorage.setItem('orcon.theme', 'dark');
    window.localStorage.setItem('orcon.density', 'compact');
    renderRoute('/settings/appearance');
    await heading('Theme');
    expect(screen.getByRole('radio', { name: 'Dark' })).toBeChecked();
    expect(screen.getByRole('radio', { name: 'Compact' })).toBeChecked();
  });

  it('marks the first useful screen once its content is shown', async () => {
    const mark = vi.spyOn(performance, 'mark');
    renderRoute('/settings/appearance');
    await heading('Theme');
    expect(mark).toHaveBeenCalledWith(FIRST_USEFUL_MARK, { detail: { route: '/settings/appearance' } });
    mark.mockRestore();
  });
});
