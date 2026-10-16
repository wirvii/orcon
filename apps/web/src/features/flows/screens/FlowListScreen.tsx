// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, FilterChip, Input, Mascot, Select } from '@orcon/ui';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { FileUp, LayoutTemplate, Plus, Search } from 'lucide-react';
import { useEffect, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { FlowRows } from '../../../app/flow-list/FlowRows';
import { orgFlowsQueryOptions, useSetFlowActive, type FlowSort } from '../../../app/flow-list/flows';
import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { QueryState } from '../../../app/query-state/QueryState';
import { CreateFlowChooser } from '../forms/CreateFlowChooser';
import type { CreateMode } from '../model/params';

const messages = defineMessages({
  search: { id: 'flows.list.search', defaultMessage: 'Search in this project', description: 'Accessible name and placeholder of the search box of the flows of a project.' },
  filterActive: { id: 'flows.list.filter.active', defaultMessage: 'Active', description: 'Filter of the flows list that keeps only the active flows.' },
  filterInactive: { id: 'flows.list.filter.inactive', defaultMessage: 'Inactive', description: 'Filter of the flows list that keeps only the inactive flows.' },
  sort: { id: 'flows.list.sort', defaultMessage: 'Order', description: 'Accessible name of the choice of the order of the flows list.' },
  sortUpdated: { id: 'flows.list.sort.updated', defaultMessage: 'Last change', description: 'Order of the flows list by when the flow last changed.' },
  sortLastRun: { id: 'flows.list.sort.lastRun', defaultMessage: 'Last run', description: 'Order of the flows list by when the flow last ran.' },
  emptyTitle: { id: 'flows.list.empty.title', defaultMessage: '{project} is ready for its first flow', description: 'Title of a project that has no flow. {project} is the name of the project.' },
  emptyText: {
    id: 'flows.list.empty.text',
    defaultMessage: 'Start from scratch, use a template or bring a flow you already have in n8n. You can move it to another project later.',
    description: 'Explains the three ways to start the first flow of a project.',
  },
  createFlow: { id: 'flows.list.create', defaultMessage: 'Create flow', description: 'Button that opens the dialog to start a flow.' },
  importN8n: { id: 'flows.list.import', defaultMessage: 'Import from n8n', description: 'Button that opens the dialog to start a flow from an n8n workflow.' },
  templates: { id: 'flows.list.templates', defaultMessage: 'See templates', description: 'Button that opens the dialog to start a flow from a template.' },
  noMatchTitle: { id: 'flows.list.noMatch.title', defaultMessage: 'No flow matches', description: 'Title of the flows list when the search or the filters leave nothing.' },
  noMatchText: { id: 'flows.list.noMatch.text', defaultMessage: 'Change the search or clear the filters to see more flows.', description: 'Says what to do when the search or the filters leave no flow.' },
});

const route = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle/flows');

/**
 * The flows tab of a project: dense rows with the last execution, the runs of the last 24 hours and the Active
 * switch, a search, two filters and an order. A project with no flow shows the thinking mascot and three ways to
 * start. `?create` opens the chooser of how to start a flow. There are no tags and no owner in the rows: the
 * contract has neither yet.
 */
export function FlowListScreen() {
  const intl = useIntl();
  const navigate = useNavigate();
  const { organization, project } = route.useRouteContext();
  const { orgSlug, projectHandle } = route.useParams();
  const search = route.useSearch();
  const sort: FlowSort = search.sort ?? '-updatedAt';
  const flows = useQuery(orgFlowsQueryOptions(organization.id, { projectId: project.id, sort, activationState: search.state, q: search.q }));
  const toggle = useSetFlowActive(organization.id);
  const [text, setText] = useState(search.q ?? '');

  // `?create` opens the chooser on arrival; closing it drops the parameter so a reload does not open it again.
  const requested: CreateMode | undefined = search.create === undefined || search.create === false ? undefined : search.create === true ? 'blank' : search.create;
  const [choosing, setChoosing] = useState<CreateMode | undefined>(requested);
  useEffect(() => {
    // The address can ask for the chooser while the page is already open (the key N, the sidebar).
    if (requested !== undefined) setChoosing(requested);
  }, [requested]);

  useEffect(() => {
    markFirstUsefulScreen('/orgs/$orgSlug/projects/$projectHandle/flows');
  }, []);

  // The search goes to the address after a short pause, so the list is not read on every key.
  useEffect(() => {
    if (text === (search.q ?? '')) return undefined;
    const timer = setTimeout(() => {
      void navigate({ to: '/orgs/$orgSlug/projects/$projectHandle/flows', params: { orgSlug, projectHandle }, search: (previous) => ({ ...previous, q: text === '' ? undefined : text }), replace: true });
    }, 250);
    return () => {
      clearTimeout(timer);
    };
  }, [text, search.q, navigate, orgSlug, projectHandle]);

  const setState = (state: 'ACTIVE' | 'INACTIVE') => {
    void navigate({
      to: '/orgs/$orgSlug/projects/$projectHandle/flows',
      params: { orgSlug, projectHandle },
      search: (previous) => ({ ...previous, state: previous.state === state ? undefined : state }),
      replace: true,
    });
  };

  const filtered = search.q !== undefined || search.state !== undefined;
  // A project that has no flow at all is one welcome card: a search box over nothing would only be in the way.
  const bare = flows.data?.length === 0 && !filtered;
  const choose = (mode: CreateMode) => () => {
    setChoosing(mode);
  };

  return (
    <div className="flex flex-col gap-3">
      {bare ? null : (
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative w-full md:w-panel-width">
          <Search aria-hidden className="text-fg-subtle pointer-events-none absolute top-1/2 left-2 size-icon-16 -translate-y-1/2" />
          <Input
            type="search"
            aria-label={intl.formatMessage(messages.search)}
            placeholder={intl.formatMessage(messages.search)}
            value={text}
            onChange={(event) => {
              setText(event.target.value);
            }}
            className="pl-8"
          />
        </div>
        <FilterChip
          label={intl.formatMessage(messages.filterActive)}
          pressed={search.state === 'ACTIVE'}
          className="max-md:hidden"
          onClick={() => {
            setState('ACTIVE');
          }}
        />
        <FilterChip
          label={intl.formatMessage(messages.filterInactive)}
          pressed={search.state === 'INACTIVE'}
          className="max-md:hidden"
          onClick={() => {
            setState('INACTIVE');
          }}
        />
        <span className="flex-1 max-md:hidden" />
        <Select
          aria-label={intl.formatMessage(messages.sort)}
          value={sort}
          onValueChange={(value) => {
            void navigate({
              to: '/orgs/$orgSlug/projects/$projectHandle/flows',
              params: { orgSlug, projectHandle },
              search: (previous) => ({ ...previous, sort: value === '-lastExecutionAt' ? '-lastExecutionAt' : undefined }),
              replace: true,
            });
          }}
          options={[
            { value: '-updatedAt', label: intl.formatMessage(messages.sortUpdated) },
            { value: '-lastExecutionAt', label: intl.formatMessage(messages.sortLastRun) },
          ]}
          className="w-panel-width max-md:hidden"
        />
      </div>
      )}
      <div className={bare ? 'border-border bg-surface rounded-lg border' : undefined}>
      <QueryState
        query={flows}
        skeletonRows={4}
        empty={
          filtered
            ? { title: intl.formatMessage(messages.noMatchTitle), description: intl.formatMessage(messages.noMatchText) }
            : {
                title: intl.formatMessage(messages.emptyTitle, { project: project.name }),
                description: intl.formatMessage(messages.emptyText),
                illustration: <Mascot pose="thinking" />,
                action: (
                  <>
                    <Button onClick={choose('blank')}>
                      <Plus aria-hidden className="size-icon-16" />
                      {intl.formatMessage(messages.createFlow)}
                    </Button>
                    <Button variant="secondary" onClick={choose('import')}>
                      <FileUp aria-hidden className="size-icon-16" />
                      {intl.formatMessage(messages.importN8n)}
                    </Button>
                    <Button variant="ghost" onClick={choose('template')}>
                      <LayoutTemplate aria-hidden className="size-icon-16" />
                      {intl.formatMessage(messages.templates)}
                    </Button>
                  </>
                ),
              }
        }
      >
        {(items) => (
          <FlowRows
            flows={items}
            orgSlug={orgSlug}
            projectHandle={projectHandle}
            busyFlowId={toggle.isPending ? toggle.variables.flow.id : undefined}
            failure={toggle.error ?? undefined}
            onToggle={(flow, active) => {
              toggle.mutate({ flow, active });
            }}
          />
        )}
      </QueryState>
      </div>
      <CreateFlowChooser
        open={choosing !== undefined}
        initialMode={choosing}
        onOpenChange={(open) => {
          if (open) return;
          setChoosing(undefined);
          if (requested !== undefined) void navigate({ to: '/orgs/$orgSlug/projects/$projectHandle/flows', params: { orgSlug, projectHandle }, search: (previous) => ({ ...previous, create: undefined }), replace: true });
        }}
        orgId={organization.id}
        orgSlug={orgSlug}
        projectHandle={projectHandle}
        replaceHistory={requested !== undefined}
      />
    </div>
  );
}
