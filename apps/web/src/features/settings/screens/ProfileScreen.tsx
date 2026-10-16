// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { currentUserQueryOptions } from '../../../app/session/current-user';
import { identityOf } from '../../../app/session/identity';
import { SettingsCard } from '../components/SettingsCard';

const messages = defineMessages({
  title: { id: 'settings.profile.title', defaultMessage: 'Profile', description: 'Heading of the card with the name and email of the person.' },
  description: {
    id: 'settings.profile.description',
    defaultMessage: 'This is how the rest of the organization sees you.',
    description: 'One line under the heading of the profile card.',
  },
  name: { id: 'settings.profile.name', defaultMessage: 'Name', description: 'Label of the name of the person on the profile card.' },
  email: { id: 'settings.profile.email', defaultMessage: 'Email address', description: 'Label of the email address of the person on the profile card.' },
  note: {
    id: 'settings.profile.note',
    defaultMessage: 'To change your details, ask whoever administers the organization.',
    description: 'Says that the profile cannot be edited here yet and who can change it.',
  },
});

/**
 * The profile of the signed-in person. Read-only until the server can change it (SPEC-019 R-UX-SET-01). The person
 * is already in the cache: the session layout reads it before any page renders.
 */
export function ProfileScreen() {
  const intl = useIntl();
  const { data: current } = useQuery(currentUserQueryOptions());

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/settings/profile');
  }, []);

  if (current === undefined) return null;
  return (
    <SettingsCard title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)}>
      <dl className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div className="flex flex-col gap-1">
          <dt className="text-sm font-medium">{intl.formatMessage(messages.name)}</dt>
          <dd className="text-fg-muted wrap-anywhere">{identityOf(current).name}</dd>
        </div>
        <div className="flex flex-col gap-1">
          <dt className="text-sm font-medium">{intl.formatMessage(messages.email)}</dt>
          <dd className="text-fg-muted wrap-anywhere">{current.email}</dd>
        </div>
      </dl>
      <p className="text-fg-muted text-sm">{intl.formatMessage(messages.note)}</p>
    </SettingsCard>
  );
}
