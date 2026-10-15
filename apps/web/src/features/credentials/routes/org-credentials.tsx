// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';
import { KeyRound } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { RouteError } from '../../../app/errors/RouteError';
import { OrgSectionPlaceholder } from '../../../app/shell/OrgSectionPlaceholder';

const messages = defineMessages({
  title: { id: 'credentials.org.title', defaultMessage: 'Credentials', description: 'Main heading of the page of credentials of the whole organization.' },
  description: {
    id: 'credentials.org.description',
    defaultMessage: 'The view of every credential of the organization is not available yet. Open a project to see and create its credentials.',
    description: 'One sentence that says the whole-organization credentials page does not exist yet and where the credentials are meanwhile.',
  },
});

function OrgCredentialsPage() {
  const intl = useIntl();
  return (
    <OrgSectionPlaceholder
      title={intl.formatMessage(messages.title)}
      description={intl.formatMessage(messages.description)}
      icon={KeyRound}
      projectPage="credentials"
      mark="/orgs/$orgSlug/credentials"
    />
  );
}

/** The credentials of the whole organization. The server lists them per project today; this page leads to those. */
export const Route = createFileRoute('/_session/orgs/$orgSlug/credentials')({
  errorComponent: RouteError,
  component: OrgCredentialsPage,
});
