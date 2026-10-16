// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, ComingSoon } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { Lock, Smartphone } from 'lucide-react';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { currentUserQueryOptions, useSignOut } from '../../../app/session/current-user';
import { SettingsCard } from '../components/SettingsCard';

const messages = defineMessages({
  sessionTitle: { id: 'settings.security.session.title', defaultMessage: 'This session', description: 'Heading of the card that shows and signs out the session of this browser.' },
  sessionText: {
    id: 'settings.security.session.text',
    defaultMessage: 'Sign out of Orcon in this browser. Other devices stay signed in.',
    description: 'Explains what signing out of this session does.',
  },
  thisBrowser: { id: 'settings.security.session.browser', defaultMessage: 'This browser', description: 'Name of the session that is open right now, the only one the server lists today.' },
  active: { id: 'settings.security.session.active', defaultMessage: 'Active now', description: 'Badge on the session that is open right now.' },
  started: { id: 'settings.security.session.started', defaultMessage: 'Started {date}', description: 'When the session began. {date} is a date and time.' },
  expires: { id: 'settings.security.session.expires', defaultMessage: 'Ends {date} at the latest', description: 'When the session ends whatever happens. {date} is a date and time.' },
  signOut: { id: 'settings.security.signOut', defaultMessage: 'Sign out of this session', description: 'Button that ends the session of this browser.' },
  comingTitle: { id: 'settings.security.coming.title', defaultMessage: 'Change password and open sessions', description: 'Title of the card that says changing the password and listing sessions are not available yet.' },
  comingText: {
    id: 'settings.security.coming.text',
    defaultMessage: 'Changing your password and seeing your other open sessions are not available yet. Whoever administers the organization can set a new password.',
    description: 'One sentence that says what is missing in password and sessions and who can help meanwhile.',
  },
});

/**
 * Password and sessions. The session of this browser is real (it comes with the signed-in person) and can be ended;
 * changing the password and listing the other sessions need operations the server does not serve yet, so the screen
 * says so (SPEC-019 R-UX-SET-02).
 */
export function SecurityScreen() {
  const intl = useIntl();
  const { data: current } = useQuery(currentUserQueryOptions());
  const signOut = useSignOut();

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/settings/security');
  }, []);

  const when = (iso: string) => intl.formatDate(iso, { dateStyle: 'medium', timeStyle: 'short' });

  return (
    <>
      <SettingsCard title={intl.formatMessage(messages.sessionTitle)} description={intl.formatMessage(messages.sessionText)}>
        {current === undefined ? null : (
          <div className="border-border flex items-center gap-3 rounded-md border p-3">
            <Smartphone aria-hidden className="text-fg-muted size-icon-20 shrink-0" />
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <b className="text-sm">{intl.formatMessage(messages.thisBrowser)}</b>
              <span className="text-fg-muted text-xs">{intl.formatMessage(messages.started, { date: when(current.currentSession.createdAt) })}</span>
              <span className="text-fg-muted text-xs">{intl.formatMessage(messages.expires, { date: when(current.currentSession.expiresAt) })}</span>
            </div>
            <Badge tone="success">{intl.formatMessage(messages.active)}</Badge>
          </div>
        )}
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
