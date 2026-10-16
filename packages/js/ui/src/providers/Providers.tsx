// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react';

import { type Density, DensityProvider } from './DensityProvider';
import { I18nProvider, type I18nProviderProps } from './I18nProvider';
import { type ThemePreference, ThemeProvider } from './ThemeProvider';
import type { Locale } from '../i18n/locales';

export interface ProvidersProps {
  children: ReactNode;
  theme?: ThemePreference | undefined;
  /** The theme of a person who has not chosen one. The app starts dark (SPEC-019 R-UX-GEN-02); without it the system decides. */
  defaultTheme?: ThemePreference | undefined;
  density?: Density | undefined;
  locale?: Locale | undefined;
  /** Messages of the application, per product locale (see I18nProvider). */
  messages?: I18nProviderProps['messages'];
}

/** Theme, density and locale in one wrapper. Props make each one controlled (stories, tests). */
export function Providers({ children, theme, defaultTheme, density, locale, messages }: ProvidersProps) {
  return (
    <ThemeProvider preference={theme} defaultPreference={defaultTheme}>
      <DensityProvider density={density}>
        <I18nProvider locale={locale} messages={messages}>{children}</I18nProvider>
      </DensityProvider>
    </ThemeProvider>
  );
}
