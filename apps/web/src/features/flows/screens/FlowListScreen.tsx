// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, Link, PageHeader, RelativeTime, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link as RouterLink } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { QueryState } from '../../../app/query-state/QueryState';
import { flowsQueryOptions } from '../api/queries';
import { CreateFlowDialog } from '../forms/CreateFlowDialog';

const messages = defineMessages({
  title: { id: 'flows.list.title', defaultMessage: 'Flows', description: 'Main heading of the list of flows of a project.' },
  description: {
    id: 'flows.list.description',
    defaultMessage: 'A flow is a process made of steps. New flows start inactive.',
    description: 'One line under the heading of the flows list: what a flow is and its starting state.',
  },
  newFlow: { id: 'flows.list.new', defaultMessage: 'New flow', description: 'Button that opens the dialog to create a flow.' },
  emptyTitle: { id: 'flows.list.empty.title', defaultMessage: 'No flows yet', description: 'Title of the flows list when the project has none.' },
  emptyDescription: {
    id: 'flows.list.empty.description',
    defaultMessage: 'Create the first flow to open it in the editor.',
    description: 'Explains the empty flows list and the next step.',
  },
  columnName: { id: 'flows.list.column.name', defaultMessage: 'Name', description: 'Column heading of the flow name, which opens the editor.' },
  columnState: { id: 'flows.list.column.state', defaultMessage: 'State', description: 'Column heading of whether the flow is active.' },
  columnUpdated: { id: 'flows.list.column.updated', defaultMessage: 'Last change', description: 'Column heading of when the flow last changed.' },
  active: { id: 'flows.list.state.active', defaultMessage: 'Active', description: 'State of a flow that runs when its triggers fire, shown as text.' },
  inactive: { id: 'flows.list.state.inactive', defaultMessage: 'Inactive', description: 'State of a flow that does not run, shown as text.' },
});

const route = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle/flows');

/** The flows of the project. The name opens the editor; the state is text, never color alone. */
export function FlowListScreen() {
  const intl = useIntl();
  const { organization, project } = route.useRouteContext();
  const flows = useQuery(flowsQueryOptions(organization.id, project.id));
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/projects/$projectHandle/flows');
  }, []);

  const newFlow = (
    <Button
      onClick={() => {
        setCreating(true);
      }}
    >
      {intl.formatMessage(messages.newFlow)}
    </Button>
  );

  return (
    <div className="flex flex-col gap-4">
      <PageHeader title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)} actions={newFlow} />
      <QueryState
        query={flows}
        empty={{ title: intl.formatMessage(messages.emptyTitle), description: intl.formatMessage(messages.emptyDescription), action: newFlow }}
      >
        {(items) => (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{intl.formatMessage(messages.columnName)}</TableHead>
                <TableHead>{intl.formatMessage(messages.columnState)}</TableHead>
                <TableHead>{intl.formatMessage(messages.columnUpdated)}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((flow) => (
                <TableRow key={flow.id}>
                  <TableCell>
                    <Link asChild>
                      <RouterLink to="/flows/$flowId" params={{ flowId: flow.id }}>
                        {flow.name}
                      </RouterLink>
                    </Link>
                  </TableCell>
                  <TableCell>{intl.formatMessage(flow.activationState === 'ACTIVE' ? messages.active : messages.inactive)}</TableCell>
                  <TableCell>
                    <RelativeTime value={flow.updatedAt} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </QueryState>
      <CreateFlowDialog open={creating} onOpenChange={setCreating} orgId={organization.id} projectId={project.id} />
    </div>
  );
}
