// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from '@tanstack/react-router';
import { useEffect, useState, type ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { clearCacheForOrganizationSwitch } from '../../../app/query-client';
import { currentUserQueryOptions, useSignOut, type OrganizationRole } from '../../../app/session/current-user';
import { identityOf } from '../../../app/session/identity';
import { useNeedsReviewCount } from '../../../app/shell/needs-review';
import { shellProjectsQueryOptions } from '../../../app/shell/shell-data';
import { rememberOrganization } from '../api/last-org';
import { organizationsQueryOptions } from '../api/queries';
import { readSidebarCollapsed, writeSidebarCollapsed } from '../api/sidebar-state';
import { AppFrame } from '../components/AppFrame';
import type { FrameUser } from '../components/UserMenu';

const messages = defineMessages({
  owner: { id: 'session.role.owner', defaultMessage: 'Owner', description: 'The role of a person who owns an organization.' },
  admin: { id: 'session.role.admin', defaultMessage: 'Administrator', description: 'The role of a person who administers an organization.' },
  member: { id: 'session.role.member', defaultMessage: 'Member', description: 'The role of an ordinary member of an organization.' },
  projectAdmin: { id: 'session.role.projectAdmin', defaultMessage: 'Project administrator', description: 'The role of a person who administers some projects of an organization.' },
  editor: { id: 'session.role.editor', defaultMessage: 'Editor', description: 'The role of a person who builds and changes flows.' },
  operator: { id: 'session.role.operator', defaultMessage: 'Operator', description: 'The role of a person who runs flows and reviews their executions.' },
  viewer: { id: 'session.role.viewer', defaultMessage: 'Viewer', description: 'The role of a person who can only look at flows and executions.' },
});

const roleMessages = {
  Owner: messages.owner,
  Admin: messages.admin,
  Member: messages.member,
  'Project Admin': messages.projectAdmin,
  Editor: messages.editor,
  Operator: messages.operator,
  Viewer: messages.viewer,
} as const satisfies Record<OrganizationRole, unknown>;

/**
 * The frame of the signed-in pages, with its data: the person, their organizations, the projects of the one being
 * used and the sidebar choice kept in this browser. The frame itself only draws what it is given.
 */
export function SessionFrame({ children }: { children: ReactNode }) {
  const intl = useIntl();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { orgSlug } = useParams({ strict: false });
  const signOut = useSignOut();
  const currentUser = useQuery(currentUserQueryOptions());
  const organizations = useQuery(organizationsQueryOptions());
  const organization = organizations.data?.find((candidate) => candidate.slug === orgSlug);
  const projects = useQuery({ ...shellProjectsQueryOptions(organization?.id ?? ''), enabled: organization !== undefined });
  const needsReviewCount = useNeedsReviewCount();

  // The person is already in the cache when the frame renders: the session layout reads it before any page.
  const userId = currentUser.data?.id;
  const [choice, setChoice] = useState<{ userId: string | undefined; collapsed: boolean | undefined }>(() => ({
    userId,
    collapsed: userId === undefined ? undefined : readSidebarCollapsed(userId),
  }));
  const collapsedChoice = choice.userId === userId ? choice.collapsed : userId === undefined ? undefined : readSidebarCollapsed(userId);

  useEffect(() => {
    if (orgSlug !== undefined) rememberOrganization(orgSlug);
  }, [orgSlug]);

  let user: FrameUser | undefined;
  if (currentUser.data !== undefined) {
    const identity = identityOf(currentUser.data);
    const role = organization === undefined ? undefined : currentUser.data.orgRoles?.find((entry) => entry.orgId === organization.id)?.role;
    user = { name: identity.name, initials: identity.initials, email: currentUser.data.email, roleLabel: role === undefined ? undefined : intl.formatMessage(roleMessages[role]) };
  }

  return (
    <AppFrame
      orgSlug={orgSlug}
      organizations={organizations.data ?? []}
      projects={projects.data}
      user={user}
      needsReviewCount={needsReviewCount}
      collapsedChoice={collapsedChoice}
      onCollapsedChange={(next) => {
        setChoice({ userId, collapsed: next });
        if (userId !== undefined) writeSidebarCollapsed(userId, next);
      }}
      onSignOut={() => {
        if (currentUser.data !== undefined) void signOut(currentUser.data.currentSession.id);
      }}
      onSwitchOrganization={(slug) => {
        // Everything read for the organization being left is dropped before the new one opens (orcon-web DT-4).
        clearCacheForOrganizationSwitch(queryClient);
        void navigate({ to: '/orgs/$orgSlug/overview', params: { orgSlug: slug } });
      }}
      onOpenOrganizations={() => {
        clearCacheForOrganizationSwitch(queryClient);
        void navigate({ to: '/orgs' });
      }}
    >
      {children}
    </AppFrame>
  );
}
