// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ChevronsUpDown, Settings } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { DropdownMenu, type MenuEntry } from '../dropdown-menu/DropdownMenu';

const messages = defineMessages({
  organizations: {
    id: 'orgSwitcher.organizations',
    defaultMessage: 'Organizations',
    description: 'Heading of the list of organizations the person belongs to, in the menu of the organization button.',
  },
  settings: {
    id: 'orgSwitcher.settings',
    defaultMessage: 'Organization settings',
    description: 'Menu entry that opens the settings of the active organization.',
  },
});

export interface OrgSwitcherOrganization {
  id: string;
  name: string;
}

export interface OrgSwitcherProps {
  organizations: readonly OrgSwitcherOrganization[];
  /** The `id` of the organization being used. */
  activeId: string;
  /** The line under the name, such as "Organization". Left out when there is nothing true to say. */
  subtitle?: string | undefined;
  /** Only the letter tile is the button, for the collapsed sidebar. */
  collapsed?: boolean | undefined;
  onSelect: (id: string) => void;
  /** The settings of the active organization. The entry is left out when it is not given. */
  settingsHref?: string | undefined;
  onOpenSettings?: (() => void) | undefined;
  /** Open the menu when the page loads. Only for stories and tests. */
  defaultOpen?: boolean | undefined;
  className?: string | undefined;
}

/** The first letter of an organization name, in capitals. */
function letterOf(name: string): string {
  return (name.trim().charAt(0) || '?').toUpperCase();
}

function Tile({ name, small = false, labelled = false }: { name: string; small?: boolean; labelled?: boolean }) {
  return (
    <span
      {...(labelled ? { role: 'img', 'aria-label': name } : { 'aria-hidden': true })}
      className={cn(
        'bg-action-primary text-action-primary-fg inline-flex shrink-0 items-center justify-center rounded-base font-bold',
        small ? 'size-icon-20 text-2xs' : 'size-icon-24 text-xs',
      )}
    >
      {letterOf(name)}
    </span>
  );
}

/**
 * The active organization, always visible at the top of the sidebar, and the list of organizations the person
 * belongs to. Choosing one switches to it; the active one has a check mark. Creating an organization is not
 * offered: organizations are made on the server.
 */
export function OrgSwitcher({ organizations, activeId, subtitle, collapsed = false, onSelect, settingsHref, onOpenSettings, defaultOpen, className }: OrgSwitcherProps) {
  const intl = useIntl();
  const active = organizations.find((organization) => organization.id === activeId) ?? organizations[0];
  if (active === undefined) return null;

  const entries: MenuEntry[] = [
    { type: 'label', label: intl.formatMessage(messages.organizations) },
    ...organizations.map(
      (organization): MenuEntry => ({
        type: 'item',
        label: organization.name,
        leading: <Tile name={organization.name} small />,
        selected: organization.id === active.id,
        onSelect: () => {
          if (organization.id !== active.id) onSelect(organization.id);
        },
      }),
    ),
    ...(settingsHref === undefined
      ? []
      : ([
          { type: 'separator' },
          { type: 'link', label: intl.formatMessage(messages.settings), href: settingsHref, icon: Settings, onSelect: onOpenSettings },
        ] satisfies MenuEntry[])),
  ];

  return (
    <DropdownMenu
      align="start"
      side="bottom"
      width="panel"
      defaultOpen={defaultOpen}
      items={entries}
      trigger={
        <button
          type="button"
          className={cn(
            'border-border hover:bg-surface-hover flex min-h-target w-full items-center gap-2 rounded-md border px-2 py-1 text-left transition-colors',
            collapsed && 'justify-center border-transparent px-0',
            className,
          )}
        >
          <Tile name={active.name} labelled={collapsed} />
          {collapsed ? null : (
            <>
              <span className="flex min-w-0 flex-1 flex-col">
                <span data-truncate className="text-fg truncate text-sm font-semibold">
                  {active.name}
                </span>
                {subtitle ? (
                  <span data-truncate className="text-fg-muted truncate text-2xs">
                    {subtitle}
                  </span>
                ) : null}
              </span>
              <ChevronsUpDown aria-hidden className="text-fg-subtle size-icon-16 shrink-0" />
            </>
          )}
        </button>
      }
    />
  );
}
