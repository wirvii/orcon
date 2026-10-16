// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, ComingSoon } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { Lock } from 'lucide-react';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { currentUserQueryOptions, useSignOut } from '../../../app/session/current-user';
import { SettingsCard } from '../components/SettingsCard';

const messages = defineMessages({
  sessionTitle: { id: 'settings.security.session.title', defaultMessage: 'This session', description: 'Heading of the card that signs out the session of this browser.' },
  sessionText: {
    id: 'settings.security.session.text',
    defaultMessage: 'Sign out of Orcon in this browser. Other devices stay signed in.',
    description: 'Explains what signing out of this session does.',
  },
  signOut: { id: 'settings.security.signOut', defaultMessage: 'Sign out of this session', description: 'Button that ends the session of this browser.' },
  comingTitle: { id: 'settings.security.coming.title', defaultMessage: 'Change password and open sessions', description: 'Title of the card that says changing the password and listing sessions are not available yet.' },
  comingText: {
    id: 'settings.security.coming.text',
    defaultMessage: 'Changing your password and seeing your other open sessions are not available yet. Whoever administers the organization can set a new password.',
    description: 'One sentence that says what is missing in password and sessions and who can help meanwhile.',
  },
});

/** Password and sessions: only ending this session exists today; the rest says so (SPEC-019 R-UX-SET-02). */
export function SecurityScreen() {
  const intl = useIntl();
  const { data: current } = useQuery(currentUserQueryOptions());
  const signOut = useSignOut();

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/settings/security');
  }, []);

  return (
    <>
      <SettingsCard title={intl.formatMessage(messages.sessionTitle)} description={intl.formatMessage(messages.sessionText)}>
        <div>
          <Button
            variant="secondary"
            disabled={current === undefined}
            onClick={() => {
              if (current !== undefined) void signOut(current.currentSession.id);
            }}
          >
            {intl.formatMessage(messages.signOut)}
          </Button>
        </div>
      </SettingsCard>
      <ComingSoon icon={Lock} title={intl.formatMessage(messages.comingTitle)} description={intl.formatMessage(messages.comingText)} />
    </>
  );
}
