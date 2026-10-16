// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Badge, Banner, Button, ComingSoon } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { Laptop, Lock, Smartphone } from 'lucide-react';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { QueryState } from '../../../app/query-state/QueryState';
import { currentUserQueryOptions, useSignOut } from '../../../app/session/current-user';
import { useRevokeOtherSession } from '../api/mutations';
import { sessionsQueryOptions } from '../api/queries';
import { SettingsCard } from '../components/SettingsCard';

const messages = defineMessages({
  sessionTitle: { id: 'settings.security.session.title', defaultMessage: 'This session', description: 'Heading of the card that shows and signs out the session of this browser.' },
  sessionText: {
    id: 'settings.security.session.text',
    defaultMessage: 'Sign out of Orcon in this browser. Other devices stay signed in.',
    description: 'Explains what signing out of this session does.',
  },
  thisBrowser: { id: 'settings.security.session.browser', defaultMessage: 'This browser', description: 'Name of the session that is open right now.' },
  active: { id: 'settings.security.session.active', defaultMessage: 'Active now', description: 'Badge on the session that is open right now.' },
  started: { id: 'settings.security.session.started', defaultMessage: 'Started {date}', description: 'When the session began. {date} is a date and time.' },
  expires: { id: 'settings.security.session.expires', defaultMessage: 'Ends {date} at the latest', description: 'When the session ends whatever happens. {date} is a date and time.' },
  signOut: { id: 'settings.security.signOut', defaultMessage: 'Sign out of this session', description: 'Button that ends the session of this browser.' },
  othersTitle: { id: 'settings.security.others.title', defaultMessage: 'Other sessions', description: 'Heading of the card that lists the other open sessions of the person.' },
  othersText: {
    id: 'settings.security.others.text',
    defaultMessage: 'Browsers and devices where you are still signed in. Signing one out ends it at once.',
    description: 'Explains the list of other open sessions and what signing one out does.',
  },
  othersEmpty: { id: 'settings.security.others.empty', defaultMessage: 'You have no other open sessions.', description: 'Says that the only open session is the one of this browser.' },
  otherName: { id: 'settings.security.others.name', defaultMessage: 'Another browser or device', description: 'Name of an open session that is not the one of this browser. The server does not say which device it is.' },
  revoke: { id: 'settings.security.others.revoke', defaultMessage: 'Sign out', description: 'Button on a row of the other sessions that ends that session.' },
  revokeNamed: {
    id: 'settings.security.others.revokeNamed',
    defaultMessage: 'Sign out the session started {date}',
    description: 'Accessible name of the button that ends one of the other sessions. {date} is the date and time it started.',
  },
  revokeFailed: {
    id: 'settings.security.others.revokeFailed',
    defaultMessage: 'The session could not be signed out. Try again.',
    description: 'Error shown when ending one of the other sessions failed.',
  },
  comingTitle: { id: 'settings.security.coming.title', defaultMessage: 'Change password', description: 'Title of the card that says changing the password is not available yet.' },
  comingText: {
    id: 'settings.security.coming.text',
    defaultMessage: 'Changing your password here is not available yet. Meanwhile an administrator can set a new one from the command line with orcon accounts set-password.',
    description: 'One sentence that says changing the password is missing and how an administrator can do it meanwhile.',
  },
});

/**
 * Password and sessions. The session of this browser can be ended, the other open sessions are listed and can be
 * ended one by one; changing the password needs an operation the server does not serve yet, so the screen says so
 * (SPEC-019 R-UX-SET-02).
 */
export function SecurityScreen() {
  const intl = useIntl();
  const { data: current } = useQuery(currentUserQueryOptions());
  const signOut = useSignOut();
  const sessions = useQuery(sessionsQueryOptions());
  const revoke = useRevokeOtherSession();

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
      <SettingsCard title={intl.formatMessage(messages.othersTitle)} description={intl.formatMessage(messages.othersText)}>
        {revoke.isError ? <Banner tone="danger" title={intl.formatMessage(messages.revokeFailed)} /> : null}
        <QueryState
          query={sessions}
          empty={{ title: intl.formatMessage(messages.othersEmpty) }}
          isEmpty={(items) => items.every((item) => item.id === current?.currentSession.id)}
          skeletonRows={2}
        >
          {(items) => (
            <ul className="flex flex-col gap-3">
              {items
                .filter((item) => item.id !== current?.currentSession.id)
                .map((item) => (
                  <li key={item.id} className="border-border flex items-center gap-3 rounded-md border p-3">
                    <Laptop aria-hidden className="text-fg-muted size-icon-20 shrink-0" />
                    <div className="flex min-w-0 flex-1 flex-col gap-1">
                      <b className="text-sm">{intl.formatMessage(messages.otherName)}</b>
                      <span className="text-fg-muted text-xs">{intl.formatMessage(messages.started, { date: when(item.createdAt) })}</span>
                      <span className="text-fg-muted text-xs">{intl.formatMessage(messages.expires, { date: when(item.expiresAt) })}</span>
                    </div>
                    <Button
                      variant="secondary"
                      loading={revoke.isPending && revoke.variables === item.id}
                      aria-label={intl.formatMessage(messages.revokeNamed, { date: when(item.createdAt) })}
                      onClick={() => {
                        revoke.mutate(item.id);
                      }}
                    >
                      {intl.formatMessage(messages.revoke)}
                    </Button>
                  </li>
                ))}
            </ul>
          )}
        </QueryState>
      </SettingsCard>
      <ComingSoon icon={Lock} title={intl.formatMessage(messages.comingTitle)} description={intl.formatMessage(messages.comingText)} />
    </>
  );
}
