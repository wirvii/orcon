// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { AppearanceControls } from '../components/AppearanceControls';
import { SettingsCard } from '../components/SettingsCard';

const messages = defineMessages({
  title: { id: 'settings.appearance.title', defaultMessage: 'Appearance', description: 'Heading of the card with the theme, density and language.' },
  description: {
    id: 'settings.appearance.description',
    defaultMessage: 'It applies right away and only to you. There is nothing to save.',
    description: 'One line under the heading of the appearance card: it applies at once and is personal.',
  },
});

/** Theme, density and language. Each choice applies at once and is kept in this browser (SPEC-019 R-UX-SET-03). */
export function AppearanceScreen() {
  const intl = useIntl();

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/settings/appearance');
  }, []);

  return (
    <SettingsCard title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)}>
      <AppearanceControls />
    </SettingsCard>
  );
}
