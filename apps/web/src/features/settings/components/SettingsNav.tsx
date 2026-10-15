// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cn } from '@orcon/ui';
import { Link as RouterLink, useParams } from '@tanstack/react-router';
import { Building2, Code, Lock, Sun, User, Users, type LucideIcon } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  label: { id: 'settings.nav.label', defaultMessage: 'Settings sections', description: 'Accessible name of the list of settings sections.' },
  account: { id: 'settings.nav.account', defaultMessage: 'Your account', description: 'Heading of the settings sections that belong to the person: profile, password and appearance.' },
  organization: {
    id: 'settings.nav.organization',
    defaultMessage: 'Organization · {name}',
    description: 'Heading of the settings sections that belong to the organization. {name} is the organization.',
  },
  profile: { id: 'settings.nav.profile', defaultMessage: 'Profile', description: 'Settings section with the name and email of the person.' },
  security: { id: 'settings.nav.security', defaultMessage: 'Password and sessions', description: 'Settings section with the password and the open sessions.' },
  appearance: { id: 'settings.nav.appearance', defaultMessage: 'Appearance', description: 'Settings section with the theme, density and language.' },
  general: { id: 'settings.nav.general', defaultMessage: 'General', description: 'Settings section with the name and address of the organization.' },
  members: { id: 'settings.nav.members', defaultMessage: 'People and roles', description: 'Settings section with the people of the organization and their roles.' },
  apiKeys: { id: 'settings.nav.apiKeys', defaultMessage: 'API keys', description: 'Settings section with the API keys of the organization.' },
});

interface Entry {
  to:
    | '/orgs/$orgSlug/settings/profile'
    | '/orgs/$orgSlug/settings/security'
    | '/orgs/$orgSlug/settings/appearance'
    | '/orgs/$orgSlug/settings/organization'
    | '/orgs/$orgSlug/settings/members'
    | '/orgs/$orgSlug/settings/api-keys';
  label: (typeof messages)[keyof typeof messages];
  icon: LucideIcon;
}

const accountEntries: Entry[] = [
  { to: '/orgs/$orgSlug/settings/profile', label: messages.profile, icon: User },
  { to: '/orgs/$orgSlug/settings/security', label: messages.security, icon: Lock },
  { to: '/orgs/$orgSlug/settings/appearance', label: messages.appearance, icon: Sun },
];

const organizationEntries: Entry[] = [
  { to: '/orgs/$orgSlug/settings/organization', label: messages.general, icon: Building2 },
  { to: '/orgs/$orgSlug/settings/members', label: messages.members, icon: Users },
  { to: '/orgs/$orgSlug/settings/api-keys', label: messages.apiKeys, icon: Code },
];

const linkClass = [
  'text-fg-muted hover:bg-surface-hover hover:text-fg flex min-h-target shrink-0 items-center gap-2 rounded-md px-2 text-sm font-medium whitespace-nowrap transition-colors',
  // On a phone the sections are a row of pills; from md up, a column of entries with their icon.
  'border-strong max-md:rounded-full max-md:border max-md:px-3',
].join(' ');

/**
 * The sections of Settings: those of the person ("Your account") and those of the organization. A column beside the
 * page from md up, a row of pills you can scroll on a phone. Only the section being shown is marked.
 */
export function SettingsNav({ organizationName }: { organizationName: string }) {
  const intl = useIntl();
  const { orgSlug } = useParams({ strict: false });
  if (orgSlug === undefined) return null;

  const group = (heading: string, entries: Entry[]) => (
    <>
      <p aria-hidden className="text-fg-subtle text-2xs px-2 pt-3 font-semibold uppercase max-md:hidden">
        {heading}
      </p>
      {entries.map(({ to, label, icon: Icon }) => (
        <RouterLink
          key={to}
          to={to}
          params={{ orgSlug }}
          className={linkClass}
          activeProps={{ 'aria-current': 'page', className: cn(linkClass, 'bg-surface-hover text-fg font-semibold') }}
        >
          <Icon aria-hidden className="size-icon-16 shrink-0 max-md:hidden" />
          <span>{intl.formatMessage(label)}</span>
        </RouterLink>
      ))}
    </>
  );

  return (
    <nav aria-label={intl.formatMessage(messages.label)} data-truncate className="flex gap-2 overflow-x-auto md:sticky md:top-0 md:flex-col md:gap-1 md:overflow-visible">
      {group(intl.formatMessage(messages.account), accountEntries)}
      {group(intl.formatMessage(messages.organization, { name: organizationName }), organizationEntries)}
    </nav>
  );
}
