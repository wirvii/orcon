// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { IntlProvider } from 'react-intl';

import { catalogs, isLocale, matchProductLocale, type Locale } from '../i18n/locales';

import { readPreference, writePreference } from './storage';

export const LOCALE_STORAGE_KEY = 'orcon.locale';

interface LocaleContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
}

const LocaleContext = createContext<LocaleContextValue | null>(null);

/** Stored preference, else the browser languages matched to en/es, else en. */
function initialLocale(): Locale {
  const saved = readPreference(LOCALE_STORAGE_KEY);
  if (isLocale(saved)) return saved;
  return matchProductLocale(typeof navigator === 'undefined' ? [] : Array.from(navigator.languages ?? []));
}

export interface I18nProviderProps {
  children: ReactNode;
  defaultLocale?: Locale;
  /** Controlled locale (Storybook globals, tests). Disables persistence. */
  locale?: Locale;
  onLocaleChange?: (locale: Locale) => void;
}

export function I18nProvider({ children, defaultLocale, locale: controlled, onLocaleChange }: I18nProviderProps) {
  const [stored, setStored] = useState<Locale>(() => defaultLocale ?? initialLocale());
  const locale = controlled ?? stored;

  useEffect(() => {
    document.documentElement.lang = locale === 'xx-LS' ? 'en' : locale;
  }, [locale]);

  const setLocale = useCallback(
    (next: Locale) => {
      if (controlled === undefined) {
        setStored(next);
        writePreference(LOCALE_STORAGE_KEY, next);
      }
      onLocaleChange?.(next);
    },
    [controlled, onLocaleChange],
  );

  const value = useMemo(() => ({ locale, setLocale }), [locale, setLocale]);

  return (
    <LocaleContext.Provider value={value}>
      <IntlProvider locale={locale} defaultLocale="en" messages={catalogs[locale]}>
        {children}
      </IntlProvider>
    </LocaleContext.Provider>
  );
}

export function useLocale(): LocaleContextValue {
  const value = useContext(LocaleContext);
  if (!value) throw new Error('useLocale must be used inside an I18nProvider');
  return value;
}
