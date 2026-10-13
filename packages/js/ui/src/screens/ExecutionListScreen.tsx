// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { Banner } from '../components/banner/Banner';
import { Button } from '../components/button/Button';
import { DataTable, type DataColumn, type SortState } from '../components/data-table/DataTable';
import { DateTime } from '../components/date-time/DateTime';
import { EmptyState } from '../components/empty-state/EmptyState';
import { Link } from '../components/link/Link';
import { NumberFormat, Duration } from '../components/number-format/NumberFormat';
import { Pagination } from '../components/pagination/Pagination';
import { RelativeTime } from '../components/relative-time/RelativeTime';
import { Skeleton } from '../components/skeleton/Skeleton';
import { StatusBadge } from '../components/status-badge/StatusBadge';

import { FIXTURE_NOW, executions as fixtureExecutions, type ExecutionRow } from './fixtures';
import { PageHeader } from './PageHeader';
import { ProductShell } from './ProductShell';

const messages = defineMessages({
  title: { id: 'screen.executions.title', defaultMessage: 'Executions', description: 'Main heading of the list of executions (runs of flows).' },
  description: {
    id: 'screen.executions.description',
    defaultMessage: 'Every run of every flow, newest first.',
    description: 'One line under the heading of the list of executions.',
  },
  tableLabel: { id: 'screen.executions.table', defaultMessage: 'Executions', description: 'Accessible name of the table of executions.' },
  flow: { id: 'screen.executions.col.flow', defaultMessage: 'Flow', description: 'Column header: the flow that ran.' },
  status: { id: 'screen.executions.col.status', defaultMessage: 'Status', description: 'Column header: how the execution is doing or ended.' },
  started: { id: 'screen.executions.col.started', defaultMessage: 'Started', description: 'Column header: when the execution started.' },
  duration: { id: 'screen.executions.col.duration', defaultMessage: 'Duration', description: 'Column header: how long the execution took.' },
  items: { id: 'screen.executions.col.items', defaultMessage: 'Items', description: 'Column header: how many items the execution handled.' },
  running: { id: 'screen.executions.stillRunning', defaultMessage: 'Not finished', description: 'Shown instead of a duration while an execution has not finished.' },
  itemsCount: {
    id: 'screen.executions.itemsCount',
    defaultMessage: '{count, plural, one {# item} other {# items}}',
    description: 'How many items an execution handled, in the phone list.',
  },
  emptyTitle: { id: 'screen.executions.empty.title', defaultMessage: 'No executions yet', description: 'Title shown when no flow has run.' },
  emptyText: {
    id: 'screen.executions.empty.text',
    defaultMessage: 'When a flow runs, each execution appears here with what every step did.',
    description: 'Text shown when no flow has run, saying what will appear.',
  },
  errorTitle: { id: 'screen.executions.error', defaultMessage: 'The executions could not be loaded.', description: 'Message shown when loading the list of executions failed.' },
  retry: { id: 'screen.executions.retry', defaultMessage: 'Try again', description: 'Button that loads the list of executions again.' },
  listLabel: { id: 'screen.executions.list', defaultMessage: 'Executions', description: 'Accessible name of the list of executions shown on a phone.' },
  loading: { id: 'screen.executions.loading', defaultMessage: 'Loading executions', description: 'Read aloud while the list of executions loads.' },
});

export interface ExecutionListScreenProps {
  executions?: readonly ExecutionRow[] | undefined;
  status?: 'ready' | 'loading' | 'error' | undefined;
  now?: string | undefined;
  onRetry?: (() => void) | undefined;
}

/**
 * S4: the list of executions, with status as icon and text, Intl dates, numbers
 * and durations, and pagination. A dense table from md up; cards on a phone. No
 * mascot belongs in this dense view.
 */
export function ExecutionListScreen({ executions = fixtureExecutions, status = 'ready', now = FIXTURE_NOW, onRetry }: ExecutionListScreenProps) {
  const intl = useIntl();
  const [sort, setSort] = useState<SortState>({ columnId: 'started', direction: 'descending' });
  const [page, setPage] = useState(1);

  const rows = [...executions].sort((a, b) => {
    const direction = sort.direction === 'ascending' ? 1 : -1;
    if (sort.columnId === 'started') return (Date.parse(a.startedAt) - Date.parse(b.startedAt)) * direction;
    if (sort.columnId === 'items') return (a.items - b.items) * direction;
    return a.flowName.localeCompare(b.flowName) * direction;
  });

  const columns: DataColumn<ExecutionRow>[] = [
    {
      id: 'flow',
      header: intl.formatMessage(messages.flow),
      sortable: true,
      cell: (row) => (
        <Link href={`#executions/${row.id}`} className="font-medium">
          {row.flowName}
          <span className="text-fg-muted ml-2 font-mono text-mono">{row.id}</span>
        </Link>
      ),
    },
    { id: 'status', header: intl.formatMessage(messages.status), cell: (row) => <StatusBadge status={row.status} /> },
    { id: 'started', header: intl.formatMessage(messages.started), sortable: true, cell: (row) => <DateTime value={row.startedAt} dateStyle="medium" timeStyle="short" /> },
    {
      id: 'duration',
      header: intl.formatMessage(messages.duration),
      cell: (row) => (row.durationMs === undefined ? <span className="text-fg-muted">{intl.formatMessage(messages.running)}</span> : <Duration ms={row.durationMs} />),
    },
    { id: 'items', header: intl.formatMessage(messages.items), align: 'end', sortable: true, cell: (row) => <NumberFormat value={row.items} /> },
  ];

  const empty = status === 'ready' && executions.length === 0;
  if (empty) {
    return (
      <ProductShell current="executions">
        <PageHeader title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)} />
        <EmptyState title={intl.formatMessage(messages.emptyTitle)} description={intl.formatMessage(messages.emptyText)} />
      </ProductShell>
    );
  }

  return (
    <ProductShell current="executions">
      <PageHeader title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)} />
      <div className="hidden md:block">
        <DataTable
          label={intl.formatMessage(messages.tableLabel)}
          columns={columns}
          rows={rows}
          getRowId={(row) => row.id}
          status={status}
          sort={sort}
          onSortChange={setSort}
          errorMessage={intl.formatMessage(messages.errorTitle)}
          onRetry={onRetry}
          stickyHeader
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
            {rows.map((row) => (
              <li key={row.id} className="bg-surface border-border flex flex-col gap-2 rounded-lg border p-3">
                <Link href={`#executions/${row.id}`} className="font-medium wrap-anywhere">
                  {row.flowName}
                </Link>
                <div className="flex flex-wrap items-center gap-2">
                  <StatusBadge status={row.status} />
                  <span className="text-fg-muted font-mono text-mono">{row.id}</span>
                </div>
                <p className="text-fg-muted text-sm wrap-anywhere">
                  <RelativeTime value={row.startedAt} now={now} />
                  {' · '}
                  {row.durationMs === undefined ? intl.formatMessage(messages.running) : <Duration ms={row.durationMs} />}
                  {' · '}
                  {intl.formatMessage(messages.itemsCount, { count: row.items })}
                </p>
              </li>
            ))}
          </ul>
        )}
      </div>
      <Pagination page={page} pageCount={3} onPageChange={setPage} className="mt-4" />
    </ProductShell>
  );
}
