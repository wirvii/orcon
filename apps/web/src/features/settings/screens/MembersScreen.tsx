// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Badge, ComingSoon } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { Users } from 'lucide-react';
import { useEffect } from 'react';
import { defineMessages, useIntl, type MessageDescriptor } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { currentUserQueryOptions, type OrganizationRole } from '../../../app/session/current-user';
import { SettingsCard } from '../components/SettingsCard';

const messages = defineMessages({
  title: { id: 'settings.members.title', defaultMessage: 'People and roles', description: 'Heading of the card that explains the roles of the organization.' },
  description: {
    id: 'settings.members.description',
    defaultMessage: 'What each role allows. The role in the organization sets what a person can do everywhere; a project role adds access to one project.',
    description: 'One line under the heading of the people and roles card.',
  },
  yourRole: { id: 'settings.members.yourRole', defaultMessage: 'Your role in {name}:', description: 'Introduces the role the signed-in person has in the organization. {name} is the organization.' },
  organizationRoles: { id: 'settings.members.organizationRoles', defaultMessage: 'Organization roles', description: 'Heading of the list of roles that apply to the whole organization.' },
  projectRoles: { id: 'settings.members.projectRoles', defaultMessage: 'Project roles', description: 'Heading of the list of roles that apply inside one project.' },
  comingTitle: { id: 'settings.members.coming.title', defaultMessage: 'The people of the organization', description: 'Title of the card that says the list of people and invitations are not available yet.' },
  comingText: {
    id: 'settings.members.coming.text',
    defaultMessage: 'Seeing the people of the organization, inviting them and changing their roles are not available yet.',
    description: 'One sentence that says what is missing in the people and roles section.',
  },
});

/** Name and one-line meaning of every predefined role (ADR-0041 section 3). */
const roles = {
  Owner: defineMessages({
    name: { id: 'settings.members.role.owner', defaultMessage: 'Owner', description: 'Name of the organization role with every permission.' },
    text: {
      id: 'settings.members.role.owner.text',
      defaultMessage: 'Everything, including deleting the organization and handing it over.',
      description: 'One line that says what the Owner role allows.',
    },
  }),
  Admin: defineMessages({
    name: { id: 'settings.members.role.admin', defaultMessage: 'Administrator', description: 'Name of the organization role that manages people and projects.' },
    text: {
      id: 'settings.members.role.admin.text',
      defaultMessage: 'Everything the owner can do, except deleting the organization or handing it over.',
      description: 'One line that says what the Administrator role allows.',
    },
  }),
  Member: defineMessages({
    name: { id: 'settings.members.role.member', defaultMessage: 'Member', description: 'Name of the organization role of an ordinary person.' },
    text: {
      id: 'settings.members.role.member.text',
      defaultMessage: 'Sees the organization and its people and can create projects. Reaches a project only through a project role.',
      description: 'One line that says what the Member role allows.',
    },
  }),
  'Project Admin': defineMessages({
    name: { id: 'settings.members.role.projectAdmin', defaultMessage: 'Project administrator', description: 'Name of the project role that manages one project.' },
    text: {
      id: 'settings.members.role.projectAdmin.text',
      defaultMessage: 'Everything inside that project.',
      description: 'One line that says what the Project administrator role allows.',
    },
  }),
  Editor: defineMessages({
    name: { id: 'settings.members.role.editor', defaultMessage: 'Editor', description: 'Name of the project role that builds flows.' },
    text: {
      id: 'settings.members.role.editor.text',
      defaultMessage: 'Creates, changes and publishes flows, and works with the executions and credentials of the project.',
      description: 'One line that says what the Editor role allows.',
    },
  }),
  Operator: defineMessages({
    name: { id: 'settings.members.role.operator', defaultMessage: 'Operator', description: 'Name of the project role that runs flows without editing them.' },
    text: {
      id: 'settings.members.role.operator.text',
      defaultMessage: 'Turns flows on and off, retries and cancels executions and resolves tasks. Does not edit flows or see message contents.',
      description: 'One line that says what the Operator role allows.',
    },
  }),
  Viewer: defineMessages({
    name: { id: 'settings.members.role.viewer', defaultMessage: 'Read only', description: 'Name of the project role that only looks.' },
    text: {
      id: 'settings.members.role.viewer.text',
      defaultMessage: 'Sees flows and executions; changes nothing.',
      description: 'One line that says what the read only role allows.',
    },
  }),
} satisfies Record<OrganizationRole, { name: MessageDescriptor; text: MessageDescriptor }>;

const organizationRoles: readonly OrganizationRole[] = ['Owner', 'Admin', 'Member'];
const projectRoles: readonly OrganizationRole[] = ['Project Admin', 'Editor', 'Operator', 'Viewer'];

const route = getRouteApi('/_session/orgs/$orgSlug');

/**
 * People and roles: the roles the product has (Owner, Administrator and Member in the organization, plus the roles of
 * a project) and what each allows, which is true today. The list of people needs an operation the server does not
 * serve yet, so the screen says so instead of faking one (SPEC-019 R-UX-SET-05, Q11).
 */
export function MembersScreen() {
  const intl = useIntl();
  const { organization } = route.useRouteContext();
  const { data: current } = useQuery(currentUserQueryOptions());
  const yourRole = current?.orgRoles?.find((entry) => entry.orgId === organization.id)?.role;

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/settings/members');
  }, []);

  const roleList = (heading: string, list: readonly OrganizationRole[]) => (
    <section className="flex flex-col gap-2" aria-label={heading}>
      <h3 className="text-fg-subtle text-2xs font-semibold uppercase">{heading}</h3>
      <ul className="grid grid-cols-1 gap-3 md:grid-cols-2">
        {list.map((role) => (
          <li key={role} className="border-border bg-surface-sunken flex flex-col gap-1 rounded-md border p-3">
            <b className="text-sm">{intl.formatMessage(roles[role].name)}</b>
            <span className="text-fg-muted text-xs">{intl.formatMessage(roles[role].text)}</span>
          </li>
        ))}
      </ul>
    </section>
  );

  return (
    <>
      <SettingsCard title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)}>
        {yourRole === undefined ? null : (
          <p className="flex flex-wrap items-center gap-2 text-sm">
            {intl.formatMessage(messages.yourRole, { name: organization.name })}
            <Badge tone="neutral">{intl.formatMessage(roles[yourRole].name)}</Badge>
          </p>
        )}
        {roleList(intl.formatMessage(messages.organizationRoles), organizationRoles)}
        {roleList(intl.formatMessage(messages.projectRoles), projectRoles)}
      </SettingsCard>
      <ComingSoon icon={Users} title={intl.formatMessage(messages.comingTitle)} description={intl.formatMessage(messages.comingText)} />
    </>
  );
}
