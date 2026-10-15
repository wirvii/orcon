// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, EmptyState } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { defineMessages, useIntl } from 'react-intl';

import { currentUserQueryOptions } from '../api/queries';
import { useSignOut } from '../api/mutations';

const messages = defineMessages({
  title: {
    id: 'session.noOrganization.title',
    defaultMessage: 'Your account is not in an organization yet',
    description: 'Heading of the page shown to a signed-in person who belongs to no active organization.',
  },
  description: {
    id: 'session.noOrganization.description',
    defaultMessage: 'An administrator of an organization has to add your account. Ask them, then sign in again.',
    description: 'Explains what to do when the signed-in person belongs to no organization.',
  },
  signOut: {
    id: 'session.noOrganization.signOut',
    defaultMessage: 'Sign out',
    description: 'Button on the page for a person without an organization that ends the session.',
  },
});

/** The start page of a signed-in person with no active organization (SPEC-016 R-SH-UI-02). */
export function NoOrganizationScreen() {
  const intl = useIntl();
  const user = useQuery(currentUserQueryOptions());
  const signOut = useSignOut();
  const sessionId = user.data?.currentSession.id;

  return (
    <EmptyState
      title={intl.formatMessage(messages.title)}
      description={intl.formatMessage(messages.description)}
      action={
        <Button
          variant="secondary"
          disabled={sessionId === undefined}
          onClick={() => {
            if (sessionId !== undefined) void signOut(sessionId);
          }}
        >
          {intl.formatMessage(messages.signOut)}
        </Button>
      }
    />
  );
}
