// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

import { readPreference, writePreference } from './storage';

export type ThemePreference = 'system' | 'light' | 'dark';
export const themePreferences: readonly ThemePreference[] = ['system', 'light', 'dark'];

export const THEME_STORAGE_KEY = 'orcon.theme';

interface ThemeContextValue {
  /** What the person chose: system, light or dark. */
  preference: ThemePreference;
  /** What is actually shown. */
  resolved: 'light' | 'dark';
  setPreference: (preference: ThemePreference) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

function isThemePreference(value: unknown): value is ThemePreference {
  return value === 'system' || value === 'light' || value === 'dark';
}

function systemTheme(): 'light' | 'dark' {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

/** Applies the preference to <html>. "system" removes the attribute so the media query decides. */
export function applyTheme(preference: ThemePreference, root: HTMLElement = document.documentElement): void {
  if (preference === 'system') {
    root.removeAttribute('data-theme');
  } else {
    root.setAttribute('data-theme', preference);
  }
}

export interface ThemeProviderProps {
  children: ReactNode;
  /** Preference used when nothing is stored. */
  defaultPreference?: ThemePreference;
  /** Controlled preference (Storybook globals, tests). Disables persistence. */
  preference?: ThemePreference;
  onPreferenceChange?: (preference: ThemePreference) => void;
}

export function ThemeProvider({
  children,
  defaultPreference = 'system',
  preference: controlled,
  onPreferenceChange,
}: ThemeProviderProps) {
  const [stored, setStored] = useState<ThemePreference>(() => {
    const saved = readPreference(THEME_STORAGE_KEY);
    return isThemePreference(saved) ? saved : defaultPreference;
  });
  const [system, setSystem] = useState<'light' | 'dark'>(systemTheme);

  const preference = controlled ?? stored;

  useEffect(() => {
    const query = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = () => setSystem(query.matches ? 'dark' : 'light');
    query.addEventListener('change', onChange);
    return () => query.removeEventListener('change', onChange);
  }, []);

  useEffect(() => {
    applyTheme(preference);
  }, [preference]);

  const setPreference = useCallback(
    (next: ThemePreference) => {
      if (controlled === undefined) {
        setStored(next);
        writePreference(THEME_STORAGE_KEY, next);
      }
      onPreferenceChange?.(next);
    },
    [controlled, onPreferenceChange],
  );

  const value = useMemo<ThemeContextValue>(
    () => ({ preference, resolved: preference === 'system' ? system : preference, setPreference }),
    [preference, system, setPreference],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const value = useContext(ThemeContext);
  if (!value) throw new Error('useTheme must be used inside a ThemeProvider');
  return value;
}
