// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { OrgSwitcher as OrgSwitcherView, useSidebarCollapsed } from '@orcon/ui';
import { useNavigate, useRouter } from '@tanstack/react-router';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  subtitle: {
    id: 'session.orgSwitcher.subtitle',
    defaultMessage: 'Organization',
    description: 'The line under the name of the active organization, at the top of the sidebar.',
  },
});

export interface FrameOrganization {
  id: string;
  slug: string;
  name: string;
}

export interface FrameOrgSwitcherProps {
  organizations: readonly FrameOrganization[];
  activeSlug: string;
  /** Another organization was chosen. The caller drops what was read for the one being left and opens the new one. */
  onSelect: (slug: string) => void;
}

/**
 * The active organization at the top of the sidebar and the organizations the person belongs to. There is no entry
 * to create an organization: they are made on the server.
 */
export function FrameOrgSwitcher({ organizations, activeSlug, onSelect }: FrameOrgSwitcherProps) {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate();
  const collapsed = useSidebarCollapsed();

  const active = organizations.find((organization) => organization.slug === activeSlug);
  if (active === undefined) return null;
  const settingsHref = router.buildLocation({ to: '/orgs/$orgSlug/settings/organization', params: { orgSlug: active.slug } }).href;

  return (
    <OrgSwitcherView
      organizations={organizations}
      activeId={active.id}
      subtitle={intl.formatMessage(messages.subtitle)}
      collapsed={collapsed}
      settingsHref={settingsHref}
      onOpenSettings={() => void navigate({ href: settingsHref })}
      onSelect={(id) => {
        const next = organizations.find((organization) => organization.id === id);
        if (next !== undefined) onSelect(next.slug);
      }}
    />
  );
}
