// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Ellipsis, LogOut, type LucideIcon } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { Avatar } from '../avatar/Avatar';
import { DropdownMenu, type MenuEntry } from '../dropdown-menu/DropdownMenu';

const messages = defineMessages({
  account: {
    id: 'userMenu.account',
    defaultMessage: 'Account menu',
    description: 'Hidden text of the avatar-only button that opens the menu of the signed-in person. The name of the person precedes it.',
  },
  signOut: {
    id: 'userMenu.signOut',
    defaultMessage: 'Sign out',
    description: 'Menu entry that ends the session of the person in this browser.',
  },
  roleInOrganization: {
    id: 'userMenu.roleInOrganization',
    defaultMessage: '{role} · {organization}',
    description: 'Second line of the account button: the role of the person and the organization it applies to.',
  },
});

export interface UserMenuItem {
  id: string;
  label: string;
  icon?: LucideIcon | undefined;
  /** Goes to an address. Without it the item only calls `onSelect`. */
  href?: string | undefined;
  /** The key that does the same thing without the menu. */
  shortcut?: string | undefined;
  onSelect?: (() => void) | undefined;
}

export interface UserMenuProps {
  /** The real name of the person, never a placeholder. */
  name: string;
  /** Up to two letters derived from the name. */
  initials: string;
  email: string;
  /** The role in the organization being used. Left out when the server does not say it: nothing is guessed. */
  roleLabel?: string | undefined;
  /** The organization the role applies to. */
  organizationName?: string | undefined;
  items: readonly UserMenuItem[];
  onSignOut: () => void;
  /** Only the avatar is the button, for the collapsed sidebar and the phone bar. */
  compact?: boolean | undefined;
  /** Where the menu opens from the button. */
  side?: 'top' | 'bottom' | 'right' | undefined;
  /** Open the menu when the page loads. Only for stories and tests. */
  defaultOpen?: boolean | undefined;
  className?: string | undefined;
}

/**
 * The account of the person who is signed in: their avatar, name and, when known, role and organization on the
 * button; their name and email, the account pages and Sign out in the menu.
 */
export function UserMenu({ name, initials, email, roleLabel, organizationName, items, onSignOut, compact = false, side = 'top', defaultOpen, className }: UserMenuProps) {
  const intl = useIntl();
  const detail =
    roleLabel !== undefined && organizationName !== undefined
      ? intl.formatMessage(messages.roleInOrganization, { role: roleLabel, organization: organizationName })
      : (roleLabel ?? organizationName);

  const entries: MenuEntry[] = [
    {
      type: 'content',
      id: 'who',
      content: (
        <div className="flex items-center gap-3">
          <Avatar name={name} initials={initials} tone="accent" decorative />
          <div className="flex min-w-0 flex-col">
            <span className="text-fg text-sm font-semibold wrap-anywhere">{name}</span>
            <span className="text-fg-muted text-xs wrap-anywhere">{email}</span>
          </div>
        </div>
      ),
    },
    { type: 'separator' },
    ...items.map((item): MenuEntry =>
      item.href === undefined
        ? { type: 'item', label: item.label, icon: item.icon, shortcut: item.shortcut, onSelect: item.onSelect }
        : { type: 'link', label: item.label, href: item.href, icon: item.icon, shortcut: item.shortcut, onSelect: item.onSelect },
    ),
    { type: 'separator' },
    { type: 'item', label: intl.formatMessage(messages.signOut), icon: LogOut, onSelect: onSignOut },
  ];

  return (
    <DropdownMenu
      align="start"
      side={side}
      width="panel"
      defaultOpen={defaultOpen}
      items={entries}
      trigger={
        <button
          type="button"
          // The name of the button is what it holds: the visible text, or for the avatar alone the name of the person
          // and "Account menu". A separate label would not contain what is drawn on it.
          className={cn(
            'hover:bg-surface-hover flex min-h-target items-center gap-3 rounded-md text-left transition-colors',
            compact ? 'min-w-target justify-center p-1' : 'w-full p-2',
            className,
          )}
        >
          <Avatar name={name} initials={initials} tone="accent" decorative={!compact} />
          {compact ? <span className="sr-only">{intl.formatMessage(messages.account)}</span> : null}
          {compact ? null : (
            <>
              <span className="flex min-w-0 flex-1 flex-col">
                <span data-truncate className="text-fg truncate text-sm font-semibold">
                  {name}
                </span>
                {detail ? (
                  <span data-truncate className="text-fg-muted truncate text-2xs">
                    {detail}
                  </span>
                ) : null}
              </span>
              <Ellipsis aria-hidden className="text-fg-subtle size-icon-16 shrink-0" />
            </>
          )}
        </button>
      }
    />
  );
}
