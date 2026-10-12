// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import en from './en.json';
import es from './es.json';
import pseudo from './xx-LS.json';

/** Product locales are en and es. xx-LS is the long-string pseudo-locale used to test text expansion. */
export type Locale = 'en' | 'es' | 'xx-LS';
export type ProductLocale = Exclude<Locale, 'xx-LS'>;

export const locales: readonly Locale[] = ['en', 'es', 'xx-LS'];
export const productLocales: readonly ProductLocale[] = ['en', 'es'];

export const catalogs: Record<Locale, Record<string, string>> = {
  en,
  es,
  'xx-LS': pseudo,
};

export function isLocale(value: unknown): value is Locale {
  return value === 'en' || value === 'es' || value === 'xx-LS';
}

/** Matches the browser's language list to a product locale; falls back to en. */
export function matchProductLocale(languages: readonly string[]): ProductLocale {
  for (const language of languages) {
    const base = language.toLowerCase().split('-')[0];
    if (base === 'es') return 'es';
    if (base === 'en') return 'en';
  }
  return 'en';
}
