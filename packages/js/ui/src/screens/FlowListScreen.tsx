// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Plus } from 'lucide-react';
import { useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { Button } from '../components/button/Button';
import { DataTable, type DataColumn } from '../components/data-table/DataTable';
import { EmptyState } from '../components/empty-state/EmptyState';
import { Link } from '../components/link/Link';
import { Mascot } from '../components/mascot/Mascot';
import { RelativeTime } from '../components/relative-time/RelativeTime';
import { StatusBadge } from '../components/status-badge/StatusBadge';
import { Switch } from '../components/switch/Switch';
import { Skeleton } from '../components/skeleton/Skeleton';
import { Banner } from '../components/banner/Banner';

import { FIXTURE_NOW, flows as fixtureFlows, type FlowRow } from './fixtures';
import { PageHeader } from './PageHeader';
import { ProductShell } from './ProductShell';

const messages = defineMessages({
  title: { id: 'screen.flows.title', defaultMessage: 'Flows', description: 'Main heading of the list of flows.' },
  description: {
    id: 'screen.flows.description',
    defaultMessage: 'Every flow you built. Turn a flow on to let it run.',
    description: 'One line under the heading of the list of flows.',
  },
  create: { id: 'screen.flows.create', defaultMessage: 'New flow', description: 'Button that starts a new flow.' },
  tableLabel: { id: 'screen.flows.table', defaultMessage: 'Flows', description: 'Accessible name of the table of flows.' },
  name: { id: 'screen.flows.col.name', defaultMessage: 'Name', description: 'Column header: the name of the flow.' },
  enabled: { id: 'screen.flows.col.enabled', defaultMessage: 'Enabled', description: 'Column header: whether the flow is turned on.' },
  lastRun: { id: 'screen.flows.col.lastRun', defaultMessage: 'Last run', description: 'Column header: when the flow last ran.' },
  lastResult: { id: 'screen.flows.col.lastResult', defaultMessage: 'Last result', description: 'Column header: how the last run ended.' },
  never: { id: 'screen.flows.neverRun', defaultMessage: 'Never run', description: 'Shown for a flow that has not run yet.' },
  on: { id: 'screen.flows.switch.on', defaultMessage: 'Enabled', description: 'Visible text of the switch of a flow that is turned on.' },
  off: { id: 'screen.flows.switch.off', defaultMessage: 'Disabled', description: 'Visible text of the switch of a flow that is turned off.' },
  switchName: {
    id: 'screen.flows.switch.name',
    defaultMessage: '{state, select, on {Enabled} other {Disabled}}: {name}',
    description: 'Accessible name of the on and off switch of a flow. It starts with the visible text.',
  },
  toggled: {
    id: 'screen.flows.toggled',
    defaultMessage: '{state, select, on {{name} is now enabled} other {{name} is now disabled}}',
    description: 'Short confirmation read aloud after a flow is turned on or off.',
  },
  emptyTitle: { id: 'screen.flows.empty.title', defaultMessage: 'No flows yet', description: 'Title shown when there are no flows.' },
  emptyText: {
    id: 'screen.flows.empty.text',
    defaultMessage: 'Create a flow to start automating. It stays off until you turn it on.',
    description: 'Text shown when there are no flows, saying what to do.',
  },
  errorTitle: {
    id: 'screen.flows.error',
    defaultMessage: 'The flows could not be loaded.',
    description: 'Message shown when loading the list of flows failed.',
  },
  retry: { id: 'screen.flows.retry', defaultMessage: 'Try again', description: 'Button that loads the list of flows again.' },
  listLabel: { id: 'screen.flows.list', defaultMessage: 'Flows', description: 'Accessible name of the list of flows shown on a phone.' },
  loading: { id: 'screen.flows.loading', defaultMessage: 'Loading flows', description: 'Read aloud while the list of flows loads.' },
});

export interface FlowListScreenProps {
  flows?: readonly FlowRow[] | undefined;
  status?: 'ready' | 'loading' | 'error' | undefined;
  /** The instant relative times are measured from. */
  now?: string | undefined;
  onCreate?: (() => void) | undefined;
  onRetry?: (() => void) | undefined;
  onToggle?: ((id: string, enabled: boolean) => void) | undefined;
}

/**
 * S2: the list of flows with an on/off switch each. A table from md up; a list of
 * cards on a phone, where every control is fully usable.
 */
export function FlowListScreen({ flows = fixtureFlows, status = 'ready', now = FIXTURE_NOW, onCreate, onRetry, onToggle }: FlowListScreenProps) {
  const intl = useIntl();
  const [overrides, setOverrides] = useState<Record<string, boolean>>({});
  const [announcement, setAnnouncement] = useState('');
  const isOn = (flow: FlowRow) => overrides[flow.id] ?? flow.enabled;
  const toggle = (flow: FlowRow, next: boolean) => {
    setOverrides((current) => ({ ...current, [flow.id]: next }));
    setAnnouncement(intl.formatMessage(messages.toggled, { state: next ? 'on' : 'off', name: flow.name }));
    onToggle?.(flow.id, next);
  };

  const flowSwitch = (flow: FlowRow) => (
    <Switch
      checked={isOn(flow)}
      onCheckedChange={(next) => toggle(flow, next)}
      aria-label={intl.formatMessage(messages.switchName, { state: isOn(flow) ? 'on' : 'off', name: flow.name })}
      label={intl.formatMessage(isOn(flow) ? messages.on : messages.off)}
    />
  );

  const columns: DataColumn<FlowRow>[] = [
    {
      id: 'name',
      header: intl.formatMessage(messages.name),
      cell: (flow) => (
        <div className="flex flex-col">
          <Link href={`#flows/${flow.id}`} className="font-medium">
            {flow.name}
          </Link>
          <span className="text-fg-muted text-sm">{flow.description}</span>
        </div>
      ),
    },
    { id: 'enabled', header: intl.formatMessage(messages.enabled), cell: flowSwitch },
    {
      id: 'lastRun',
      header: intl.formatMessage(messages.lastRun),
      cell: (flow) => (flow.lastRunAt ? <RelativeTime value={flow.lastRunAt} now={now} /> : <span className="text-fg-muted">{intl.formatMessage(messages.never)}</span>),
    },
    { id: 'lastResult', header: intl.formatMessage(messages.lastResult), cell: (flow) => (flow.lastStatus ? <StatusBadge status={flow.lastStatus} /> : null) },
  ];

  const create = (
    <Button onClick={onCreate}>
      <Plus aria-hidden className="size-icon-16" />
      {intl.formatMessage(messages.create)}
    </Button>
  );

  const empty = status === 'ready' && flows.length === 0;

  return (
    <ProductShell current="flows">
      <PageHeader title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)} actions={create} />
      <div role="status" className="sr-only">
        {announcement}
      </div>
      {empty ? (
        <EmptyState
          title={intl.formatMessage(messages.emptyTitle)}
          description={intl.formatMessage(messages.emptyText)}
          mascot={<Mascot pose="happy" />}
          action={create}
        />
      ) : (
        <>
          <div className="hidden md:block">
            <DataTable
              label={intl.formatMessage(messages.tableLabel)}
              columns={columns}
              rows={flows}
              getRowId={(flow) => flow.id}
              status={status}
              errorMessage={intl.formatMessage(messages.errorTitle)}
              onRetry={onRetry}
            />
          </div>
          <div className="md:hidden">
            {status === 'loading' ? (
              <div className="flex flex-col gap-3" aria-busy>
                <span className="sr-only">{intl.formatMessage(messages.loading)}</span>
                {[0, 1, 2].map((index) => (
                  <Skeleton key={index} shape="block" className="w-full" />
                ))}
              </div>
            ) : status === 'error' ? (
              <Banner tone="danger" title={intl.formatMessage(messages.errorTitle)} action={<Button variant="secondary" onClick={onRetry}>{intl.formatMessage(messages.retry)}</Button>} />
            ) : (
              <ul aria-label={intl.formatMessage(messages.listLabel)} className="flex flex-col gap-3">
                {flows.map((flow) => (
                  <li key={flow.id} className="bg-surface border-border flex flex-col gap-2 rounded-lg border p-3">
                    <Link href={`#flows/${flow.id}`} className="font-medium wrap-anywhere">
                      {flow.name}
                    </Link>
                    <p className="text-fg-muted text-sm wrap-anywhere">{flow.description}</p>
                    <div className="flex flex-wrap items-center gap-2">
                      {flow.lastStatus ? <StatusBadge status={flow.lastStatus} /> : null}
                      <span className="text-fg-muted text-sm">
                        {flow.lastRunAt ? <RelativeTime value={flow.lastRunAt} now={now} /> : intl.formatMessage(messages.never)}
                      </span>
                    </div>
                    {flowSwitch(flow)}
                  </li>
                ))}
              </ul>
            )}
          </div>
        </>
      )}
    </ProductShell>
  );
}
