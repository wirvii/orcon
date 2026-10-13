// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { act, render, screen } from '@testing-library/react';
import { FormattedMessage, defineMessages } from 'react-intl';
import { describe, expect, it, vi } from 'vitest';

import { catalogs, matchProductLocale } from '../../src/i18n/locales';
import { DensityProvider, useDensity } from '../../src/providers/DensityProvider';
import { I18nProvider, useLocale } from '../../src/providers/I18nProvider';
import { ThemeProvider, useTheme } from '../../src/providers/ThemeProvider';

function ThemeProbe() {
  const { preference, resolved, setPreference } = useTheme();
  return (
    <button type="button" onClick={() => setPreference('dark')}>
      {preference}:{resolved}
    </button>
  );
}

describe('ThemeProvider', () => {
  it('sets data-theme for light and dark and removes it for system', () => {
    const { rerender } = render(
      <ThemeProvider preference="dark">
        <ThemeProbe />
      </ThemeProvider>,
    );
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark');
    rerender(
      <ThemeProvider preference="light">
        <ThemeProbe />
      </ThemeProvider>,
    );
    expect(document.documentElement).toHaveAttribute('data-theme', 'light');
    rerender(
      <ThemeProvider preference="system">
        <ThemeProbe />
      </ThemeProvider>,
    );
    expect(document.documentElement).not.toHaveAttribute('data-theme');
  });

  it('resolves "system" from the color-scheme media query', () => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() })),
    );
    render(
      <ThemeProvider>
        <ThemeProbe />
      </ThemeProvider>,
    );
    expect(screen.getByRole('button')).toHaveTextContent('system:dark');
  });

  it('stores a chosen preference and restores it', () => {
    const first = render(
      <ThemeProvider>
        <ThemeProbe />
      </ThemeProvider>,
    );
    act(() => screen.getByRole('button').click());
    expect(window.localStorage.getItem('orcon.theme')).toBe('dark');
    first.unmount();
    render(
      <ThemeProvider>
        <ThemeProbe />
      </ThemeProvider>,
    );
    expect(screen.getByRole('button')).toHaveTextContent('dark:dark');
  });

  it('ignores an invalid stored value', () => {
    window.localStorage.setItem('orcon.theme', 'purple');
    render(
      <ThemeProvider>
        <ThemeProbe />
      </ThemeProvider>,
    );
    expect(screen.getByRole('button')).toHaveTextContent('system:light');
  });

  it('does not persist when controlled', () => {
    const onChange = vi.fn();
    render(
      <ThemeProvider preference="light" onPreferenceChange={onChange}>
        <ThemeProbe />
      </ThemeProvider>,
    );
    act(() => screen.getByRole('button').click());
    expect(onChange).toHaveBeenCalledWith('dark');
    expect(window.localStorage.getItem('orcon.theme')).toBeNull();
    expect(document.documentElement).toHaveAttribute('data-theme', 'light');
  });

  it('throws outside a provider', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    expect(() => render(<ThemeProbe />)).toThrow(/ThemeProvider/);
  });
});

function DensityProbe() {
  const { density, setDensity } = useDensity();
  return (
    <button type="button" onClick={() => setDensity('compact')}>
      {density}
    </button>
  );
}

describe('DensityProvider', () => {
  it('marks compact on <html> and leaves comfortable unmarked', () => {
    const { rerender } = render(
      <DensityProvider density="compact">
        <DensityProbe />
      </DensityProvider>,
    );
    expect(document.documentElement).toHaveAttribute('data-density', 'compact');
    rerender(
      <DensityProvider density="comfortable">
        <DensityProbe />
      </DensityProvider>,
    );
    expect(document.documentElement).not.toHaveAttribute('data-density');
  });

  it('stores a chosen density', () => {
    render(
      <DensityProvider>
        <DensityProbe />
      </DensityProvider>,
    );
    act(() => screen.getByRole('button').click());
    expect(window.localStorage.getItem('orcon.density')).toBe('compact');
    expect(document.documentElement).toHaveAttribute('data-density', 'compact');
  });
});

const messages = defineMessages({
  probe: { id: 'common.spinner.loading', defaultMessage: 'Loading', description: 'Probe message for the provider test.' },
});

function LocaleProbe() {
  const { locale, setLocale } = useLocale();
  return (
    <button type="button" onClick={() => setLocale('es')}>
      {locale}: <FormattedMessage {...messages.probe} />
    </button>
  );
}

describe('I18nProvider', () => {
  it('formats messages with the active catalog and sets the document language', () => {
    render(
      <I18nProvider locale="es">
        <LocaleProbe />
      </I18nProvider>,
    );
    expect(screen.getByRole('button')).toHaveTextContent(`es: ${catalogs.es['common.spinner.loading']}`);
    expect(document.documentElement).toHaveAttribute('lang', 'es');
  });

  it('uses the long-string pseudo-locale and reports English as the document language', () => {
    render(
      <I18nProvider locale="xx-LS">
        <LocaleProbe />
      </I18nProvider>,
    );
    const pseudo = catalogs['xx-LS']['common.spinner.loading'];
    expect(pseudo).toBeDefined();
    expect(pseudo?.length).toBeGreaterThan((catalogs.en['common.spinner.loading'] ?? '').length * 1.3);
    expect(document.documentElement).toHaveAttribute('lang', 'en');
  });

  it('falls back to the browser language, then to English', () => {
    expect(matchProductLocale(['fr-FR', 'es-CO', 'en'])).toBe('es');
    expect(matchProductLocale(['en-US'])).toBe('en');
    expect(matchProductLocale(['de', 'ja'])).toBe('en');
    expect(matchProductLocale([])).toBe('en');
  });

  it('stores a chosen locale', () => {
    render(
      <I18nProvider defaultLocale="en">
        <LocaleProbe />
      </I18nProvider>,
    );
    act(() => screen.getByRole('button').click());
    expect(window.localStorage.getItem('orcon.locale')).toBe('es');
  });
});
