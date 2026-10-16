// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Badge, ComingSoon } from '@orcon/ui';
import { getRouteApi } from '@tanstack/react-router';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';

const messages = defineMessages({
  title: { id: 'project.settings.title', defaultMessage: 'Settings', description: 'Heading of the settings tab of a project.' },
  name: { id: 'project.settings.name', defaultMessage: 'Name', description: 'Label of the name of a project in its settings.' },
  handle: { id: 'project.settings.handle', defaultMessage: 'Address name', description: 'Label of the readable name used in the address of a project.' },
  environment: { id: 'project.settings.environment', defaultMessage: 'Environment', description: 'Label of the optional environment of a project.' },
  noEnvironment: { id: 'project.settings.noEnvironment', defaultMessage: 'No environment', description: 'Shown when a project has no environment label.' },
  protection: { id: 'project.settings.protection', defaultMessage: 'Protection', description: 'Label of whether a project asks for confirmation before changing production.' },
  protectedOn: { id: 'project.settings.protected', defaultMessage: 'Protected', description: 'The project asks for confirmation for actions that change production behavior.' },
  protectedOff: { id: 'project.settings.notProtected', defaultMessage: 'Not protected', description: 'The project does not ask for confirmation before changing production.' },
  soonTitle: { id: 'project.settings.soon.title', defaultMessage: 'Edit the project', description: 'Title of the card that says changing a project is not available yet.' },
  soonText: {
    id: 'project.settings.soon.text',
    defaultMessage: 'Renaming a project, changing its protection and managing who can see it are not available yet.',
    description: 'One sentence that says what is missing in the settings of a project.',
  },
});

const route = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle/settings');

/** What the project is called and how it is protected, read only: the server cannot change a project yet (SPEC-019 F-04). */
export function ProjectSettingsScreen() {
  const intl = useIntl();
  const { project } = route.useRouteContext();
  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/projects/$projectHandle/settings');
  }, []);
  return (
    <div className="flex flex-col gap-4">
      <h2 className="text-lg font-semibold">{intl.formatMessage(messages.title)}</h2>
      <dl className="border-border bg-surface grid grid-cols-1 gap-x-6 gap-y-3 rounded-lg border p-4 md:grid-cols-4">
        <dt className="text-fg-muted">{intl.formatMessage(messages.name)}</dt>
        <dd className="font-medium wrap-anywhere md:col-span-3">{project.name}</dd>
        <dt className="text-fg-muted">{intl.formatMessage(messages.handle)}</dt>
        <dd className="font-mono text-sm md:col-span-3">{project.handle}</dd>
        <dt className="text-fg-muted">{intl.formatMessage(messages.environment)}</dt>
        <dd className="md:col-span-3">{project.environment ?? intl.formatMessage(messages.noEnvironment)}</dd>
        <dt className="text-fg-muted">{intl.formatMessage(messages.protection)}</dt>
        <dd className="md:col-span-3">
          <Badge tone="neutral">{intl.formatMessage(project.protected ? messages.protectedOn : messages.protectedOff)}</Badge>
        </dd>
      </dl>
      <ComingSoon title={intl.formatMessage(messages.soonTitle)} description={intl.formatMessage(messages.soonText)} />
    </div>
  );
}
