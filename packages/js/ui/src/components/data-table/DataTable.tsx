// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ArrowDown, ArrowUp, ChevronsUpDown, CircleAlert } from 'lucide-react';
import type { ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { Button } from '../button/Button';
import { Skeleton } from '../skeleton/Skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../table/Table';

const messages = defineMessages({
  loading: { id: 'dataTable.loading', defaultMessage: 'Loading rows', description: 'Screen reader text while the rows of a table load.' },
  empty: { id: 'dataTable.empty', defaultMessage: 'No results', description: 'Shown in a table that has no rows.' },
  error: { id: 'dataTable.error', defaultMessage: 'The rows could not be loaded.', description: 'Shown in a table when loading its rows failed.' },
  retry: { id: 'dataTable.retry', defaultMessage: 'Try again', description: 'Button that loads the rows of a table again.' },
});

export type SortDirection = 'ascending' | 'descending';

export interface SortState {
  columnId: string;
  direction: SortDirection;
}

export interface DataColumn<Row> {
  id: string;
  /** Column header text. */
  header: string;
  /** Renders the cell. Return text or a component; use StatusBadge for status. */
  cell: (row: Row) => ReactNode;
  sortable?: boolean | undefined;
  align?: 'start' | 'end' | undefined;
}

export interface DataTableProps<Row> {
  /** Names the table for assistive technology. */
  label: string;
  columns: readonly DataColumn<Row>[];
  rows: readonly Row[];
  getRowId: (row: Row) => string;
  /** `ready` shows the rows; `loading` shows placeholder rows; `error` shows a message with a retry. */
  status?: 'ready' | 'loading' | 'error' | undefined;
  sort?: SortState | undefined;
  /** Sorting is done by the caller (usually on the server). */
  onSortChange?: ((sort: SortState) => void) | undefined;
  /** Text for an empty table. Defaults to a generic one; say what to do when you can. */
  emptyMessage?: ReactNode | undefined;
  /** Text for a failed load. */
  errorMessage?: ReactNode | undefined;
  onRetry?: (() => void) | undefined;
  /** Keep the header visible while the body scrolls. Set a height on the table with `className`, such as `max-h-panel-width`. */
  stickyHeader?: boolean | undefined;
  /** How many placeholder rows to show while loading. */
  loadingRows?: number | undefined;
  className?: string | undefined;
}

/**
 * A table of data with sortable columns and the loading, empty and error states
 * a list needs. Row height follows the density tokens. The table scrolls inside
 * a focusable, labeled region so keyboard users can reach every row.
 */
export function DataTable<Row>({
  label,
  columns,
  rows,
  getRowId,
  status = 'ready',
  sort,
  onSortChange,
  emptyMessage,
  errorMessage,
  onRetry,
  stickyHeader = false,
  loadingRows = 3,
  className,
}: DataTableProps<Row>) {
  const intl = useIntl();

  const toggle = (column: DataColumn<Row>) => {
    if (!onSortChange) return;
    const active = sort?.columnId === column.id;
    onSortChange({ columnId: column.id, direction: active && sort.direction === 'ascending' ? 'descending' : 'ascending' });
  };

  const span = columns.length;

  return (
    <div
      role="region"
      aria-label={label}
      tabIndex={0}
      className={cn('bg-surface border-border overflow-auto rounded-md border', className)}
    >
      <Table aria-busy={status === 'loading' || undefined}>
        <TableHeader>
          <TableRow header>
            {columns.map((column) => {
              const active = sort?.columnId === column.id;
              const SortIcon = !active ? ChevronsUpDown : sort.direction === 'ascending' ? ArrowUp : ArrowDown;
              return (
                <TableHead
                  key={column.id}
                  sticky={stickyHeader}
                  aria-sort={column.sortable ? (active ? sort.direction : 'none') : undefined}
                  className={column.align === 'end' ? 'text-right' : undefined}
                >
                  {column.sortable ? (
                    <Button variant="ghost" onClick={() => toggle(column)} className="-mx-control-x text-fg-muted text-sm font-medium focus-visible:-outline-offset-2">
                      {column.header}
                      <SortIcon aria-hidden className="size-icon-16" />
                    </Button>
                  ) : (
                    column.header
                  )}
                </TableHead>
              );
            })}
          </TableRow>
        </TableHeader>
        <TableBody>
          {status === 'loading'
            ? Array.from({ length: loadingRows }, (_, index) => (
                <TableRow key={`loading-${index}`}>
                  {columns.map((column, cellIndex) => (
                    <TableCell key={column.id}>
                      {cellIndex === 0 ? <span className="sr-only">{intl.formatMessage(messages.loading)}</span> : null}
                      <Skeleton shape="text" className="w-full" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            : null}
          {status === 'error' ? (
            <TableRow>
              <TableCell colSpan={span}>
                <div role="alert" className="text-status-danger flex flex-wrap items-center gap-2">
                  <CircleAlert aria-hidden className="size-icon-16 shrink-0" />
                  <span>{errorMessage ?? intl.formatMessage(messages.error)}</span>
                  {onRetry ? (
                    <Button variant="secondary" onClick={onRetry}>
                      {intl.formatMessage(messages.retry)}
                    </Button>
                  ) : null}
                </div>
              </TableCell>
            </TableRow>
          ) : null}
          {status === 'ready' && rows.length === 0 ? (
            <TableRow>
              <TableCell colSpan={span} className="text-fg-muted">
                {emptyMessage ?? intl.formatMessage(messages.empty)}
              </TableCell>
            </TableRow>
          ) : null}
          {status === 'ready'
            ? rows.map((row) => (
                <TableRow key={getRowId(row)}>
                  {columns.map((column) => (
                    <TableCell key={column.id} className={column.align === 'end' ? 'text-right' : undefined}>
                      {column.cell(row)}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            : null}
        </TableBody>
      </Table>
    </div>
  );
}
