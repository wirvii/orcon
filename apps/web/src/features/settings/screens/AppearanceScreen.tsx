// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Link, PageHeader } from '@orcon/ui';
import { getRouteApi, Link as RouterLink } from '@tanstack/react-router';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { PreferencesForm } from '../forms/preferences-form';
import { sections, type Section } from '../model/preferences';

const messages = defineMessages({
  title: {
    id: 'settings.appearance.title',
    defaultMessage: 'Appearance and language',
    description: 'Main heading of the settings page for theme, density and language.',
  },
  description: {
    id: 'settings.appearance.description',
    defaultMessage: 'These choices are saved in this browser.',
    description: 'One line under the heading of the appearance settings: where the choices are kept.',
  },
  onThisPage: {
    id: 'settings.appearance.onThisPage',
    defaultMessage: 'On this page',
    description: 'Accessible name of the list of links that jump to the sections of the appearance settings.',
  },
  theme: { id: 'settings.appearance.nav.theme', defaultMessage: 'Theme', description: 'Link to the theme section of the appearance settings.' },
  density: { id: 'settings.appearance.nav.density', defaultMessage: 'Density', description: 'Link to the density section of the appearance settings.' },
  language: { id: 'settings.appearance.nav.language', defaultMessage: 'Language', description: 'Link to the language section of the appearance settings.' },
});

const route = getRouteApi('/settings/appearance');

const navMessages: Record<Section, (typeof messages)['theme']> = {
  theme: messages.theme,
  density: messages.density,
  language: messages.language,
};

/** The appearance and language settings. The section in the address decides which part takes focus. */
export function AppearanceScreen() {
  const intl = useIntl();
  const { section } = route.useSearch();

  useEffect(() => {
    markFirstUsefulScreen('/settings/appearance');
  }, []);

  return (
    <div className="flex flex-col gap-2">
      <PageHeader title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)} />
      <nav aria-label={intl.formatMessage(messages.onThisPage)}>
        <ul className="flex flex-wrap gap-x-4">
          {sections.map((name) => (
            <li key={name}>
              <Link asChild>
                <RouterLink to="/settings/appearance" search={{ section: name }} aria-current={section === name ? 'location' : undefined}>
                  {intl.formatMessage(navMessages[name])}
                </RouterLink>
              </Link>
            </li>
          ))}
        </ul>
      </nav>
      <PreferencesForm section={section} />
    </div>
  );
}
