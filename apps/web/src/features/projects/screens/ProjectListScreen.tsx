// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, Link, Mascot, PageHeader, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link as RouterLink, useNavigate } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { QueryState } from '../../../app/query-state/QueryState';
import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { projectsQueryOptions } from '../api/queries';
import { CreateProjectDialog } from '../forms/CreateProjectDialog';

const messages = defineMessages({
  title: {
    id: 'projects.list.title',
    defaultMessage: 'Projects',
    description: 'Main heading of the list of projects of an organization.',
  },
  description: {
    id: 'projects.list.description',
    defaultMessage: 'A project groups the flows of one area of work.',
    description: 'One line under the heading of the projects list: what a project is for.',
  },
  newProject: { id: 'projects.list.new', defaultMessage: 'New project', description: 'Button that opens the dialog to create a project.' },
  emptyTitle: { id: 'projects.list.empty.title', defaultMessage: 'No projects yet', description: 'Title of the projects list when there is none.' },
  emptyDescription: {
    id: 'projects.list.empty.description',
    defaultMessage: 'Create the first project to start building flows.',
    description: 'Explains the empty projects list and the next step.',
  },
  columnName: { id: 'projects.list.column.name', defaultMessage: 'Name', description: 'Column heading of the project name.' },
  columnFlows: { id: 'projects.list.column.flows', defaultMessage: 'Flows', description: 'Column heading of how many flows a project has.' },
  columnEnvironment: { id: 'projects.list.column.environment', defaultMessage: 'Environment', description: 'Column heading of the optional environment label.' },
  columnHandle: { id: 'projects.list.column.handle', defaultMessage: 'Address name', description: 'Column heading of the readable name used in the address of a project.' },
  noEnvironment: { id: 'projects.list.noEnvironment', defaultMessage: 'No environment', description: 'Shown in the environment column when a project has no environment label.' },
});

const route = getRouteApi('/_session/orgs/$orgSlug');
const listRoute = getRouteApi('/_session/orgs/$orgSlug/projects');

/** The projects of the organization the person can read. Each name leads to the project's flows. */
export function ProjectListScreen() {
  const intl = useIntl();
  const { organization } = route.useRouteContext();
  const { orgSlug } = route.useParams();
  const projects = useQuery(projectsQueryOptions(organization.id));
  const navigate = useNavigate();
  const { create } = listRoute.useSearch();
  // `?create` opens the dialog on arrival; closing it drops the parameter so a reload does not open it again.
  const [creating, setCreating] = useState(create === true);
  useEffect(() => {
    // The address can ask for the dialog while the page is already open (the key N, the sidebar).
    if (create === true) setCreating(true);
  }, [create]);

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/projects');
  }, []);

  const newProject = (
    <Button
      onClick={() => {
        setCreating(true);
      }}
    >
      {intl.formatMessage(messages.newProject)}
    </Button>
  );

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)} actions={newProject} />
      <QueryState
        query={projects}
        empty={{
          title: intl.formatMessage(messages.emptyTitle),
          description: intl.formatMessage(messages.emptyDescription),
          action: newProject,
          illustration: <Mascot pose="happy" />,
        }}
      >
        {(items) => (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{intl.formatMessage(messages.columnName)}</TableHead>
                <TableHead>{intl.formatMessage(messages.columnFlows)}</TableHead>
                <TableHead>{intl.formatMessage(messages.columnEnvironment)}</TableHead>
                <TableHead>{intl.formatMessage(messages.columnHandle)}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((project) => (
                <TableRow key={project.id}>
                  <TableCell>
                    <Link asChild>
                      <RouterLink to="/orgs/$orgSlug/projects/$projectHandle/flows" params={{ orgSlug, projectHandle: project.handle }}>
                        {project.name}
                      </RouterLink>
                    </Link>
                  </TableCell>
                  <TableCell className="text-fg-muted tabular-nums">{project.flowCount ?? ''}</TableCell>
                  <TableCell className="text-fg-muted">{project.environment ?? intl.formatMessage(messages.noEnvironment)}</TableCell>
                  <TableCell className="font-mono text-sm">{project.handle}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </QueryState>
      <CreateProjectDialog
        open={creating}
        onOpenChange={(open) => {
          setCreating(open);
          if (!open && create === true) void navigate({ to: '/orgs/$orgSlug/projects', params: { orgSlug }, search: {}, replace: true });
        }}
        orgId={organization.id}
        orgSlug={orgSlug}
      />
    </div>
  );
}
