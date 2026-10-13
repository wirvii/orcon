// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, useIntl } from 'react-intl';

import { productLocales, type ProductLocale } from '../../i18n/locales';
import { useLocale } from '../../providers/I18nProvider';
import { Select } from '../select/Select';

const messages = defineMessages({
  label: { id: 'locale.label', defaultMessage: 'Language', description: 'Accessible name of the control that chooses the interface language.' },
});

/** The name of a language in that language, from Intl: "English", "Español". */
export function languageName(locale: ProductLocale): string {
  const name = new Intl.DisplayNames([locale], { type: 'language' }).of(locale) ?? locale;
  return name.charAt(0).toLocaleUpperCase(locale) + name.slice(1);
}

export interface LocaleSwitcherProps {
  /** Controlled value. Without it the switcher reads and sets the I18nProvider locale. */
  value?: ProductLocale | undefined;
  onValueChange?: ((value: ProductLocale) => void) | undefined;
  className?: string | undefined;
}

/** Chooses the interface language. Each language is listed by its own name, so it is findable whatever the current language. */
export function LocaleSwitcher({ value, onValueChange, className }: LocaleSwitcherProps) {
  const intl = useIntl();
  const state = useLocale();
  const current = value ?? (state.locale === 'xx-LS' ? 'en' : state.locale);
  return (
    <Select
      aria-label={intl.formatMessage(messages.label)}
      value={current}
      onValueChange={(next) => {
        const locale = next as ProductLocale;
        if (onValueChange) onValueChange(locale);
        else state.setLocale(locale);
      }}
      options={productLocales.map((locale) => ({ value: locale, label: languageName(locale) }))}
      className={className}
    />
  );
}
