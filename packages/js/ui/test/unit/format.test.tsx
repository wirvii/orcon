// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { CodeBlock } from '../../src/components/code-block/CodeBlock';
import { DateTime } from '../../src/components/date-time/DateTime';
import { DensitySwitcher } from '../../src/components/density-switcher/DensitySwitcher';
import { LocaleSwitcher, languageName } from '../../src/components/locale-switcher/LocaleSwitcher';
import { Duration, NumberFormat, formatDuration } from '../../src/components/number-format/NumberFormat';
import { RelativeTime, relativeParts } from '../../src/components/relative-time/RelativeTime';
import { ScrollArea } from '../../src/components/scroll-area/ScrollArea';
import { ThemeSwitcher } from '../../src/components/theme-switcher/ThemeSwitcher';
import { Toaster } from '../../src/components/toast/Toast';
import { DensityProvider } from '../../src/providers/DensityProvider';
import { I18nProvider } from '../../src/providers/I18nProvider';
import { ThemeProvider } from '../../src/providers/ThemeProvider';

import { renderWithProviders } from './helpers';

afterEach(() => vi.restoreAllMocks());

describe('relativeParts', () => {
  const minute = 60_000;
  it('picks seconds, minutes, hours, days, months and years', () => {
    expect(relativeParts(-30_000)).toEqual({ value: -30, unit: 'second' });
    expect(relativeParts(-5 * minute)).toEqual({ value: -5, unit: 'minute' });
    expect(relativeParts(2 * 60 * minute)).toEqual({ value: 2, unit: 'hour' });
    expect(relativeParts(-3 * 24 * 60 * minute)).toEqual({ value: -3, unit: 'day' });
    expect(relativeParts(-60 * 24 * 60 * minute)).toEqual({ value: -2, unit: 'month' });
    expect(relativeParts(-800 * 24 * 60 * minute)).toEqual({ value: -2, unit: 'year' });
  });

  it('never rolls over to a unit it has not reached', () => {
    expect(relativeParts(59_000).unit).toBe('second');
    expect(relativeParts(60_000).unit).toBe('minute');
    expect(relativeParts(-59 * minute).unit).toBe('minute');
    expect(relativeParts(-60 * minute).unit).toBe('hour');
  });

  it('treats "now" as zero seconds', () => {
    expect(relativeParts(0)).toEqual({ value: 0, unit: 'second' });
  });
});

describe('RelativeTime', () => {
  const now = '2026-03-14T15:00:00.000Z';

  it('formats with Intl in the active locale and keeps the ISO time on hover', () => {
    renderWithProviders(<RelativeTime now={now} value="2026-03-14T14:55:00.000Z" />);
    const time = screen.getByText('5 minutes ago');
    expect(time).toHaveAttribute('datetime', '2026-03-14T14:55:00.000Z');
    expect(time).toHaveAttribute('title', '2026-03-14T14:55:00.000Z');
  });

  it('uses the Spanish plural and phrasing', () => {
    renderWithProviders(<RelativeTime now={now} value="2026-03-14T14:55:00.000Z" />, { locale: 'es' });
    expect(screen.getByText(/hace 5 minutos/)).toBeInTheDocument();
  });

  it('says "in" for the future', () => {
    renderWithProviders(<RelativeTime now={now} value="2026-03-14T17:00:00.000Z" />);
    expect(screen.getByText('in 2 hours')).toBeInTheDocument();
  });

  it('renders nothing for an invalid date', () => {
    const { container } = renderWithProviders(<RelativeTime now={now} value="not a date" />);
    expect(container.querySelector('time')).toBeNull();
  });
});

describe('DateTime', () => {
  it('formats with Intl and exposes the ISO timestamp', () => {
    renderWithProviders(<DateTime value="2026-03-14T15:09:26.000Z" />);
    const time = document.querySelector('time');
    expect(time).toHaveAttribute('datetime', '2026-03-14T15:09:26.000Z');
    expect(time).toHaveAttribute('title', '2026-03-14T15:09:26.000Z');
    expect(time?.textContent).toMatch(/2026/);
  });

  it('follows the locale', () => {
    renderWithProviders(<DateTime value="2026-03-14T15:09:26.000Z" dateStyle="long" />, { locale: 'es' });
    expect(document.querySelector('time')?.textContent).toMatch(/marzo/);
  });

  it('can show only the time, or the time zone name', () => {
    const { unmount } = renderWithProviders(<DateTime value="2026-03-14T15:09:26.000Z" timeOnly />);
    expect(document.querySelector('time')?.textContent).not.toMatch(/2026/);
    unmount();
    renderWithProviders(<DateTime value="2026-03-14T15:09:26.000Z" timeStyle="short" showTimeZone />);
    expect(document.querySelector('time')?.textContent).toMatch(/[A-Z]{2,5}|GMT|UTC/);
  });

  it('renders nothing for an invalid date', () => {
    renderWithProviders(<DateTime value="nope" />);
    expect(document.querySelector('time')).toBeNull();
  });
});

describe('NumberFormat and Duration', () => {
  it('formats decimals, percentages, currency and compact numbers per locale', () => {
    renderWithProviders(
      <>
        <NumberFormat value={1234567.891} maximumFractionDigits={2} />
        <NumberFormat value={0.256} format="percent" />
        <NumberFormat value={1234.5} format="currency" currency="USD" />
        <NumberFormat value={1234567} compact />
      </>,
    );
    expect(screen.getByText('1,234,567.89')).toBeInTheDocument();
    expect(screen.getByText('26%')).toBeInTheDocument();
    expect(screen.getByText('$1,234.50')).toBeInTheDocument();
    expect(screen.getByText('1.2M')).toBeInTheDocument();
  });

  it('uses the Spanish separators', () => {
    renderWithProviders(<NumberFormat value={1234567.891} maximumFractionDigits={2} />, { locale: 'es' });
    expect(screen.getByText('1.234.567,89')).toBeInTheDocument();
  });

  it('formats durations with units, at most two parts', () => {
    expect(formatDuration(7_500_000, 'en')).toMatch(/2 hr.*5 min/);
    expect(formatDuration(45_000, 'en')).toMatch(/45 sec/);
    expect(formatDuration(0, 'en')).toMatch(/0 sec/);
    expect(formatDuration(90_061_000, 'en', 3)).toMatch(/1 day.*1 hr.*1 min/);
  });

  it('Duration follows the locale', () => {
    renderWithProviders(<Duration ms={7_500_000} />, { locale: 'es' });
    expect(document.body.textContent).toMatch(/2\s?h.*5\s?min/);
  });
});

describe('switchers', () => {
  it('ThemeSwitcher reads and sets the ThemeProvider preference', async () => {
    render(
      <I18nProvider locale="en">
        <ThemeProvider defaultPreference="system">
          <ThemeSwitcher />
        </ThemeProvider>
      </I18nProvider>,
    );
    expect(screen.getByRole('radiogroup', { name: 'Theme' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'System' })).toBeChecked();
    await userEvent.click(screen.getByRole('radio', { name: 'Dark' }));
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark');
    expect(window.localStorage.getItem('orcon.theme')).toBe('dark');
  });

  it('DensitySwitcher reads and sets the DensityProvider density', async () => {
    render(
      <I18nProvider locale="en">
        <DensityProvider>
          <DensitySwitcher />
        </DensityProvider>
      </I18nProvider>,
    );
    expect(screen.getByRole('radio', { name: 'Comfortable' })).toBeChecked();
    await userEvent.click(screen.getByRole('radio', { name: 'Compact' }));
    expect(document.documentElement).toHaveAttribute('data-density', 'compact');
  });

  it('switchers are translated', () => {
    renderWithProviders(<ThemeSwitcher value="light" onValueChange={() => undefined} />, { locale: 'es' });
    expect(screen.getByRole('radiogroup', { name: 'Tema' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'Oscuro' })).toBeInTheDocument();
  });

  it('LocaleSwitcher lists each language by its own name', async () => {
    expect(languageName('en')).toBe('English');
    expect(languageName('es')).toBe('Español');
    const onChange = vi.fn();
    renderWithProviders(<LocaleSwitcher value="en" onValueChange={onChange} />);
    const trigger = screen.getByRole('combobox', { name: 'Language' });
    expect(trigger).toHaveTextContent('English');
  });
});

describe('CodeBlock, ScrollArea and Toaster', () => {
  it('CodeBlock is a labeled, focusable region so keyboard users can scroll it', () => {
    renderWithProviders(<CodeBlock label="Request body" code={'{"a":1}'} />);
    const region = screen.getByRole('region', { name: 'Request body' });
    expect(region).toHaveAttribute('tabindex', '0');
    expect(region).toHaveTextContent('{"a":1}');
  });

  it('CodeBlock copies its code and confirms', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    renderWithProviders(<CodeBlock label="Request body" code="GET /orders" copyable />);
    await userEvent.click(screen.getByRole('button', { name: 'Copy code' }));
    expect(writeText).toHaveBeenCalledWith('GET /orders');
    expect(await screen.findByText('Copied')).toBeInTheDocument();
  });

  it('CodeBlock has no copy button unless asked', () => {
    renderWithProviders(<CodeBlock label="Request body" code="x" />);
    expect(screen.queryByRole('button')).toBeNull();
  });

  it('ScrollArea is a labeled, focusable region', () => {
    renderWithProviders(
      <ScrollArea label="Recent executions">
        <p>Item</p>
      </ScrollArea>,
    );
    expect(screen.getByRole('region', { name: 'Recent executions' })).toHaveAttribute('tabindex', '0');
  });

  it('Toaster names its notification region in the active language', () => {
    renderWithProviders(<Toaster />, { locale: 'es' });
    // The library appends the shortcut hint (Alt+T) to the translated name.
    expect(document.querySelector('section')?.getAttribute('aria-label')).toMatch(/^Notificaciones/);
  });
});
