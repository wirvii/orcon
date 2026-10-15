// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { UserMenu as UserMenuView, type UserMenuItem } from '@orcon/ui';
import { useNavigate, useRouter } from '@tanstack/react-router';
import { Building2, Code, Keyboard, Sun, User } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  profile: { id: 'session.userMenu.profile', defaultMessage: 'My profile', description: 'Menu entry that opens the profile page of the signed-in person.' },
  appearance: { id: 'session.userMenu.appearance', defaultMessage: 'Appearance', description: 'Menu entry that opens the appearance settings: theme, density and language.' },
  apiKeys: { id: 'session.userMenu.apiKeys', defaultMessage: 'API keys', description: 'Menu entry that opens the page of API keys of the organization.' },
  shortcuts: { id: 'session.userMenu.shortcuts', defaultMessage: 'Keyboard shortcuts', description: 'Menu entry that opens the list of keyboard shortcuts.' },
  switchOrganization: {
    id: 'session.userMenu.switchOrganization',
    defaultMessage: 'Switch organization',
    description: 'Menu entry that opens the list of organizations. Shown only to people with more than one, on pages outside an organization.',
  },
});

/** The signed-in person as the frame shows them. */
export interface FrameUser {
  /** The real name, never the placeholder "Owner". */
  name: string;
  initials: string;
  email: string;
  /** The role in the organization being used, worded for the person. Left out when the server does not say it. */
  roleLabel?: string | undefined;
}

export interface FrameUserMenuProps {
  user: FrameUser;
  /** The organization being used: its name is shown with the role and the settings pages are in it. */
  organization?: { slug: string; name: string } | undefined;
  /** Only the avatar is the button. */
  compact?: boolean | undefined;
  side?: 'top' | 'bottom' | undefined;
  onSignOut: () => void;
  onOpenShortcuts?: (() => void) | undefined;
  /** Opens the list of organizations. Offered on pages outside an organization, to a person who belongs to several. */
  onSwitchOrganization?: (() => void) | undefined;
}

/**
 * The account of the signed-in person: their name, initials, role when the server says it, the account pages and
 * Sign out.
 */
export function FrameUserMenu({ user, organization, compact = false, side = 'top', onSignOut, onOpenShortcuts, onSwitchOrganization }: FrameUserMenuProps) {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate();

  const settingsItem = (id: string, label: string, icon: UserMenuItem['icon'], section: 'profile' | 'appearance' | 'api-keys'): UserMenuItem | undefined => {
    if (organization === undefined) return undefined;
    const href = router.buildLocation({ to: `/orgs/$orgSlug/settings/${section}`, params: { orgSlug: organization.slug } }).href;
    return { id, label, icon, href, onSelect: () => void navigate({ href }) };
  };

  const items = [
    settingsItem('profile', intl.formatMessage(messages.profile), User, 'profile'),
    settingsItem('appearance', intl.formatMessage(messages.appearance), Sun, 'appearance'),
    settingsItem('api-keys', intl.formatMessage(messages.apiKeys), Code, 'api-keys'),
    onSwitchOrganization === undefined ? undefined : { id: 'switch', label: intl.formatMessage(messages.switchOrganization), icon: Building2, onSelect: onSwitchOrganization },
    onOpenShortcuts === undefined ? undefined : { id: 'shortcuts', label: intl.formatMessage(messages.shortcuts), icon: Keyboard, shortcut: '?', onSelect: onOpenShortcuts },
  ].filter((item): item is UserMenuItem => item !== undefined);

  return (
    <UserMenuView
      name={user.name}
      initials={user.initials}
      email={user.email}
      roleLabel={user.roleLabel}
      organizationName={organization?.name}
      items={items}
      compact={compact}
      side={side}
      onSignOut={onSignOut}
    />
  );
}
