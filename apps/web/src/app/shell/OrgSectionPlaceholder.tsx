// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ComingSoon, ListRow, PageHeader } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, useNavigate, useRouter } from '@tanstack/react-router';
import type { LucideIcon } from 'lucide-react';
import { useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../perf/marks';
import { QueryState } from '../query-state/QueryState';
import { shellProjectsQueryOptions } from './shell-data';

const messages = defineMessages({
  projectsHeading: {
    id: 'app.orgSection.projectsHeading',
    defaultMessage: 'Meanwhile, open a project',
    description: 'Heading above the list of projects on an organization-wide page that is not available yet: its content can be reached through each project.',
  },
  noProjects: { id: 'app.orgSection.noProjects', defaultMessage: 'No projects yet', description: 'Title of the list of projects when the organization has none.' },
  noProjectsText: {
    id: 'app.orgSection.noProjectsText',
    defaultMessage: 'Create a project from the Projects heading of the sidebar to start.',
    description: 'Explains the empty list of projects and where to create one.',
  },
});

const route = getRouteApi('/_session/orgs/$orgSlug');

export type ProjectPage = 'credentials' | 'executions';

export interface OrgSectionPlaceholderProps {
  title: string;
  /** One sentence that says what is missing. */
  description: string;
  icon: LucideIcon;
  /** The page of each project that has this section's content, offered as the way in while the whole-organization page is missing. */
  projectPage?: ProjectPage | undefined;
  /** The route id of the page, for the first-useful-screen mark. */
  mark: string;
}

/**
 * An organization-wide page whose whole-organization view does not exist yet (templates, credentials, executions):
 * an honest "coming soon" card and, where the same content already exists per project, the projects to open it from.
 * Nothing is faked and nothing is an error.
 */
export function OrgSectionPlaceholder({ title, description, icon, projectPage, mark }: OrgSectionPlaceholderProps) {
  const intl = useIntl();
  const router = useRouter();
  const navigate = useNavigate();
  const { organization } = route.useRouteContext();
  const { orgSlug } = route.useParams();
  const projects = useQuery({ ...shellProjectsQueryOptions(organization.id), enabled: projectPage !== undefined });

  useEffect(() => {
    markFirstUsefulScreen(mark);
  }, [mark]);

  const hrefOf = (handle: string) =>
    router.buildLocation({ to: `/orgs/$orgSlug/projects/$projectHandle/${projectPage ?? 'credentials'}`, params: { orgSlug, projectHandle: handle } }).href;

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title={title} breadcrumb={[{ label: organization.name }, { label: title }]} />
      <ComingSoon title={title} description={description} icon={icon} headingLevel={2} />
      {projectPage === undefined ? null : (
        <section aria-labelledby="org-section-projects" className="flex flex-col gap-2">
          <h2 id="org-section-projects" className="text-md font-semibold">
            {intl.formatMessage(messages.projectsHeading)}
          </h2>
          <QueryState query={projects} empty={{ title: intl.formatMessage(messages.noProjects), description: intl.formatMessage(messages.noProjectsText) }}>
            {(items) => (
              <div className="border-border bg-surface rounded-lg border">
                {items.map((project) => (
                  <ListRow
                    key={project.id}
                    href={hrefOf(project.handle)}
                    title={project.name}
                    onNavigate={() => void navigate({ href: hrefOf(project.handle) })}
                  />
                ))}
              </div>
            )}
          </QueryState>
        </section>
      )}
    </div>
  );
}
