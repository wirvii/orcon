// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, Link, Mascot, PageHeader, RelativeTime } from '@orcon/ui';
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
        empty={{
          title: intl.formatMessage(messages.emptyTitle),
          description: intl.formatMessage(messages.emptyDescription),
          action: newFlow,
          illustration: <Mascot pose="happy" />,
        }}
      >
        {(items) => (
          // A row on a computer, a card on a phone: the same markup, laid out by the width. The labels of the
          // columns show only on a phone, where the card has no column heads.
          <ul className="border-border flex flex-col rounded-lg border">
            {items.map((flow) => (
              <li key={flow.id} className="border-border flex flex-col gap-1 border-b p-4 last:border-b-0 md:grid md:grid-cols-3 md:items-center md:gap-4">
                <span className="font-medium wrap-anywhere">
                  <Link asChild>
                    <RouterLink to="/flows/$flowId" params={{ flowId: flow.id }}>
                      {flow.name}
                    </RouterLink>
                  </Link>
                </span>
                <span>
                  <span className="text-fg-muted md:hidden">{intl.formatMessage(messages.columnState)}: </span>
                  {intl.formatMessage(flow.activationState === 'ACTIVE' ? messages.active : messages.inactive)}
                </span>
                <span className="text-fg-muted">
                  <span className="md:hidden">{intl.formatMessage(messages.columnUpdated)}: </span>
                  <RelativeTime value={flow.updatedAt} />
                </span>
              </li>
            ))}
          </ul>
        )}
      </QueryState>
      <CreateFlowDialog open={creating} onOpenChange={setCreating} orgId={organization.id} projectId={project.id} />
    </div>
  );
}
