// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, useIntl } from 'react-intl';

import { DensitySwitcher } from '../components/density-switcher/DensitySwitcher';
import { LocaleSwitcher } from '../components/locale-switcher/LocaleSwitcher';
import { ThemeSwitcher } from '../components/theme-switcher/ThemeSwitcher';

import { PageHeader } from './PageHeader';
import { ProductShell } from './ProductShell';

const messages = defineMessages({
  title: { id: 'screen.appearance.title', defaultMessage: 'Appearance and language', description: 'Main heading of the settings page for theme, density and language.' },
  description: {
    id: 'screen.appearance.description',
    defaultMessage: 'These choices are saved in this browser and apply right away.',
    description: 'One line under the heading of the appearance settings: where the choices are kept.',
  },
  themeTitle: { id: 'screen.appearance.theme.title', defaultMessage: 'Theme', description: 'Heading of the section that chooses light, dark or the system theme.' },
  themeText: {
    id: 'screen.appearance.theme.text',
    defaultMessage: 'System follows the setting of your device.',
    description: 'Explains the theme options: system follows the device.',
  },
  densityTitle: { id: 'screen.appearance.density.title', defaultMessage: 'Density', description: 'Heading of the section that chooses how much fits on a screen.' },
  densityText: {
    id: 'screen.appearance.density.text',
    defaultMessage: 'Compact fits more rows on screen. Comfortable is easier to read and to tap.',
    description: 'Explains the two densities: compact and comfortable.',
  },
  languageTitle: { id: 'screen.appearance.language.title', defaultMessage: 'Language', description: 'Heading of the section that chooses the language of the interface.' },
  languageText: {
    id: 'screen.appearance.language.text',
    defaultMessage: 'Dates, times and numbers follow the language you choose.',
    description: 'Explains that formats follow the chosen language.',
  },
});

/**
 * S9: appearance and language. The three choices (theme, density, language) read and
 * write the providers, so the screen changes the moment a choice is made.
 */
export function AppearanceSettingsScreen() {
  const intl = useIntl();
  return (
    <ProductShell current="settings">
      <PageHeader title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)} />
      <div className="flex max-w-screen-md flex-col gap-4">
        <section aria-labelledby="appearance-theme" className="bg-surface border-border flex flex-col gap-2 rounded-lg border p-4">
          <h2 id="appearance-theme" className="text-lg font-semibold break-words">
            {intl.formatMessage(messages.themeTitle)}
          </h2>
          <p className="text-fg-muted break-words">{intl.formatMessage(messages.themeText)}</p>
          <ThemeSwitcher />
        </section>
        <section aria-labelledby="appearance-density" className="bg-surface border-border flex flex-col gap-2 rounded-lg border p-4">
          <h2 id="appearance-density" className="text-lg font-semibold break-words">
            {intl.formatMessage(messages.densityTitle)}
          </h2>
          <p className="text-fg-muted break-words">{intl.formatMessage(messages.densityText)}</p>
          <DensitySwitcher />
        </section>
        <section aria-labelledby="appearance-language" className="bg-surface border-border flex flex-col gap-2 rounded-lg border p-4">
          <h2 id="appearance-language" className="text-lg font-semibold break-words">
            {intl.formatMessage(messages.languageTitle)}
          </h2>
          <p className="text-fg-muted break-words">{intl.formatMessage(messages.languageText)}</p>
          <LocaleSwitcher />
        </section>
      </div>
    </ProductShell>
  );
}
