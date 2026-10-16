// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';
import { Activity } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { RouteError } from '../../../app/errors/RouteError';
import { OrgSectionPlaceholder } from '../../../app/shell/OrgSectionPlaceholder';

const messages = defineMessages({
  title: { id: 'executions.org.title', defaultMessage: 'Executions', description: 'Main heading of the page of executions of the whole organization.' },
  description: {
    id: 'executions.org.description',
    defaultMessage: 'The view of every execution of the organization is not available yet. Open a project to see its executions.',
    description: 'One sentence that says the whole-organization executions page does not exist yet and where the executions are meanwhile.',
  },
});

function OrgExecutionsPage() {
  const intl = useIntl();
  return (
    <OrgSectionPlaceholder
      title={intl.formatMessage(messages.title)}
      description={intl.formatMessage(messages.description)}
      icon={Activity}
      projectPage="executions"
      mark="/orgs/$orgSlug/executions"
    />
  );
}

/** The executions of the whole organization. The server lists them per project today; this page leads to those. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/executions')({
  errorComponent: RouteError,
  component: OrgExecutionsPage,
});
