// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Avatar } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { QueryState } from '../../../app/query-state/QueryState';
import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { currentUserQueryOptions } from '../../../app/session/current-user';
import { identityOf } from '../../../app/session/identity';
import { profileQueryOptions } from '../api/queries';
import { SettingsCard } from '../components/SettingsCard';
import { DisplayNameForm } from '../forms/DisplayNameForm';

const messages = defineMessages({
  title: { id: 'settings.profile.title', defaultMessage: 'Profile', description: 'Heading of the card with the name and email of the person.' },
  description: {
    id: 'settings.profile.description',
    defaultMessage: 'This is how the rest of the organization sees you.',
    description: 'One line under the heading of the profile card.',
  },
  email: { id: 'settings.profile.email', defaultMessage: 'Email address', description: 'Label of the email address of the person on the profile card.' },
  note: {
    id: 'settings.profile.note',
    defaultMessage: 'The email address cannot be changed here. Ask whoever administers the organization.',
    description: 'Says the email address is not editable on the profile card and who can change it.',
  },
});

/**
 * The profile of the signed-in person: the name can be changed, the email cannot (SPEC-019 R-UX-SET-01). The person
 * is already in the cache for the header; the form reads them again to get the entity tag the change must send.
 */
export function ProfileScreen() {
  const intl = useIntl();
  const { data: current } = useQuery(currentUserQueryOptions());
  const profile = useQuery(profileQueryOptions());

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/settings/profile');
  }, []);

  if (current === undefined) return null;
  return (
    <SettingsCard title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)}>
      <Avatar name={identityOf(current).name} initials={identityOf(current).initials} size="lg" tone="accent" decorative />
      <QueryState query={profile} empty={{ title: intl.formatMessage(messages.title) }} isEmpty={() => false} skeletonRows={1}>
        {(loaded) => (
          <DisplayNameForm
            profile={loaded}
            onReload={async () => (await profile.refetch()).data}
          />
        )}
      </QueryState>
      <dl className="grid grid-cols-1 gap-4">
        <div className="flex flex-col gap-1">
          <dt className="text-sm font-medium">{intl.formatMessage(messages.email)}</dt>
          <dd className="text-fg-muted wrap-anywhere">{current.email}</dd>
        </div>
      </dl>
      <p className="text-fg-muted text-sm">{intl.formatMessage(messages.note)}</p>
    </SettingsCard>
  );
}
