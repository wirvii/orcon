// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Link, PageHeader } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { Link as RouterLink } from '@tanstack/react-router';
import { defineMessages, useIntl } from 'react-intl';

import { QueryState } from '../../../app/query-state/QueryState';
import { organizationsQueryOptions } from '../api/queries';

const messages = defineMessages({
  title: {
    id: 'session.orgs.title',
    defaultMessage: 'Choose an organization',
    description: 'Main heading of the organization picker.',
  },
  description: {
    id: 'session.orgs.description',
    defaultMessage: 'Pick the organization you want to work in. Each one keeps its own projects and flows.',
    description: 'Explains the organization picker: each organization has its own projects.',
  },
  emptyTitle: {
    id: 'session.orgs.empty.title',
    defaultMessage: 'No organizations yet',
    description: 'Title of the empty picker, for a person who belongs to no organization.',
  },
  emptyDescription: {
    id: 'session.orgs.empty.description',
    defaultMessage: 'An administrator of an organization has to add your account.',
    description: 'Explains how to get into an organization when the picker is empty.',
  },
  slug: {
    id: 'session.orgs.slug',
    defaultMessage: 'Address name {slug}',
    description: 'Shows the readable name of an organization used in its addresses.',
  },
});

/** Lists the organizations of the signed-in person. Each one leads to its projects. */
export function OrganizationPickerScreen() {
  const intl = useIntl();
  const organizations = useQuery(organizationsQueryOptions());

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)} />
      <QueryState
        query={organizations}
        empty={{ title: intl.formatMessage(messages.emptyTitle), description: intl.formatMessage(messages.emptyDescription) }}
      >
        {(items) => (
          <ul className="flex flex-col gap-2">
            {items.map((organization) => (
              <li key={organization.id}>
                <Link asChild>
                  <RouterLink
                    to="/orgs/$orgSlug"
                    params={{ orgSlug: organization.slug }}
                    className="bg-surface border-border hover:bg-surface-sunken focus-visible:outline-focus flex flex-col gap-1 rounded-lg border p-4 wrap-anywhere"
                  >
                    <span className="text-lg font-semibold">{organization.name}</span>
                    <span className="text-fg-muted text-sm">{intl.formatMessage(messages.slug, { slug: organization.slug })}</span>
                  </RouterLink>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </QueryState>
    </div>
  );
}
