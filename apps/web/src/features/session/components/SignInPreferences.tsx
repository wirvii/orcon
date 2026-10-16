// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, DropdownMenu, IconButton, languageName, productLocales, useLocale, useTheme, type MenuEntry } from '@orcon/ui';
import { Globe, Moon, Sun } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  toLight: { id: 'session.signIn.prefs.toLight', defaultMessage: 'Switch to the light theme', description: 'Accessible name of the button on the sign-in page that changes the theme from dark to light.' },
  toDark: { id: 'session.signIn.prefs.toDark', defaultMessage: 'Switch to the dark theme', description: 'Accessible name of the button on the sign-in page that changes the theme from light to dark.' },
});

/**
 * Language and theme, two small controls at the top right of the sign-in page (SPEC-019 R-UX-LGN-03). A person who
 * is not signed in can still choose them, and they apply at once and are kept in this browser. The language button
 * shows the language in use and opens the two to choose from; the theme button flips between dark and light.
 */
export function SignInPreferences() {
  const intl = useIntl();
  const language = useLocale();
  const theme = useTheme();
  const dark = theme.resolved === 'dark';
  // The pseudo-locale of the design system tests is not a language a person can choose.
  const current = language.locale === 'es' ? 'es' : 'en';

  const languages: MenuEntry[] = productLocales.map((locale) => ({
    type: 'item',
    label: languageName(locale),
    selected: locale === current,
    onSelect: () => {
      language.setLocale(locale);
    },
  }));

  return (
    <div className="flex items-center gap-1">
      <DropdownMenu
        align="end"
        items={languages}
        trigger={
          <Button variant="ghost">
            <Globe aria-hidden className="size-icon-16" />
            {languageName(current)}
          </Button>
        }
      />
      <IconButton
        variant="ghost"
        icon={dark ? Sun : Moon}
        label={intl.formatMessage(dark ? messages.toLight : messages.toDark)}
        onClick={() => {
          theme.setPreference(dark ? 'light' : 'dark');
        }}
      />
    </div>
  );
}
