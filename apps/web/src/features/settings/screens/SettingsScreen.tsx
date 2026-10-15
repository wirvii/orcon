// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { PageHeader } from '@orcon/ui';
import { getRouteApi, Outlet } from '@tanstack/react-router';
import { defineMessages, useIntl } from 'react-intl';

import { SettingsNav } from '../components/SettingsNav';

const messages = defineMessages({
  title: { id: 'settings.title', defaultMessage: 'Settings', description: 'Main heading of the settings pages.' },
});

const organizationRoute = getRouteApi('/_session/orgs/$orgSlug');

/** The frame of every settings page: the heading and the list of sections beside the section being shown. */
export function SettingsScreen() {
  const intl = useIntl();
  const { organization } = organizationRoute.useRouteContext();
  return (
    <div className="flex flex-col gap-2">
      <PageHeader title={intl.formatMessage(messages.title)} breadcrumb={[{ label: organization.name }, { label: intl.formatMessage(messages.title) }]} />
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:gap-8">
        <div className="md:w-panel-width md:shrink-0">
          <SettingsNav organizationName={organization.name} />
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-5">
          <Outlet />
        </div>
      </div>
    </div>
  );
}
