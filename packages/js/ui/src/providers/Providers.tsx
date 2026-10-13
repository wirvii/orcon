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
  density?: Density | undefined;
  locale?: Locale | undefined;
  /** Messages of the application, per product locale (see I18nProvider). */
  messages?: I18nProviderProps['messages'];
}

/** Theme, density and locale in one wrapper. Props make each one controlled (stories, tests). */
export function Providers({ children, theme, density, locale, messages }: ProvidersProps) {
  return (
    <ThemeProvider preference={theme}>
      <DensityProvider density={density}>
        <I18nProvider locale={locale} messages={messages}>{children}</I18nProvider>
      </DensityProvider>
    </ThemeProvider>
  );
}
