// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Avatar, Button, DropdownMenu, type MenuEntry } from '@orcon/ui';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { defineMessages, useIntl } from 'react-intl';

import { clearCacheForOrganizationSwitch } from '../../../app/query-client';
import { useSignOut } from '../api/mutations';
import { currentUserQueryOptions, organizationsQueryOptions } from '../api/queries';

const messages = defineMessages({
  account: {
    id: 'session.userMenu.account',
    defaultMessage: 'Account menu',
    description: 'Accessible name of the button that opens the menu of the signed-in person.',
  },
  switchOrganization: {
    id: 'session.userMenu.switchOrganization',
    defaultMessage: 'Switch organization',
    description: 'Menu entry that opens the list of organizations. Shown only to people with more than one.',
  },
  signOut: {
    id: 'session.userMenu.signOut',
    defaultMessage: 'Sign out',
    description: 'Menu entry that ends the session of the person in this browser.',
  },
});

/**
 * The menu of the person who is signed in: name and email, switching organization when there is
 * more than one, and sign out. Renders nothing until the current user is known.
 */
export function UserMenu() {
  const intl = useIntl();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useQuery(currentUserQueryOptions());
  const organizations = useQuery(organizationsQueryOptions());
  const signOut = useSignOut();

  const current = user.data;
  if (current === undefined) return null;

  const items: MenuEntry[] = [
    { type: 'label', label: current.displayName },
    { type: 'label', label: current.email },
    { type: 'separator' },
    ...((organizations.data?.length ?? 0) > 1
      ? [
          {
            type: 'item' as const,
            label: intl.formatMessage(messages.switchOrganization),
            onSelect: () => {
              // Everything read for the organization being left is dropped before the picker opens (orcon-web DT-4).
              clearCacheForOrganizationSwitch(queryClient);
              void navigate({ to: '/orgs' });
            },
          },
        ]
      : []),
    { type: 'item', label: intl.formatMessage(messages.signOut), onSelect: () => void signOut(current.currentSession.id) },
  ];

  return (
    <DropdownMenu
      align="end"
      items={items}
      trigger={
        <Button variant="ghost" aria-label={intl.formatMessage(messages.account)}>
          <Avatar name={current.displayName} />
        </Button>
      }
    />
  );
}
