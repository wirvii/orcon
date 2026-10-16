// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { getRouteApi } from '@tanstack/react-router';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { SettingsCard } from '../components/SettingsCard';

const messages = defineMessages({
  title: { id: 'settings.organization.title', defaultMessage: 'Organization', description: 'Heading of the card with the name and address of the organization.' },
  description: {
    id: 'settings.organization.description',
    defaultMessage: 'Details of the organization that everyone in it sees.',
    description: 'One line under the heading of the organization card.',
  },
  name: { id: 'settings.organization.name', defaultMessage: 'Name', description: 'Label of the name of the organization.' },
  address: { id: 'settings.organization.address', defaultMessage: 'Web address', description: 'Label of the short name of the organization used in its address.' },
  note: {
    id: 'settings.organization.note',
    defaultMessage: 'You cannot change these details here yet. Changing the web address would break the links people saved.',
    description: 'Says the organization details are read-only for now and why the address matters.',
  },
});

const route = getRouteApi('/_session/orgs/$orgSlug');

/** The organization, read-only until the server can change it (SPEC-019 R-UX-SET-04). */
export function OrganizationScreen() {
  const intl = useIntl();
  const { organization } = route.useRouteContext();

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/settings/organization');
  }, []);

  return (
    <SettingsCard title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)}>
      <dl className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div className="flex flex-col gap-1">
          <dt className="text-sm font-medium">{intl.formatMessage(messages.name)}</dt>
          <dd className="text-fg-muted wrap-anywhere">{organization.name}</dd>
        </div>
        <div className="flex flex-col gap-1">
          <dt className="text-sm font-medium">{intl.formatMessage(messages.address)}</dt>
          <dd className="text-fg-muted font-mono wrap-anywhere">{organization.slug}</dd>
        </div>
      </dl>
      <p className="text-fg-muted text-sm">{intl.formatMessage(messages.note)}</p>
    </SettingsCard>
  );
}
