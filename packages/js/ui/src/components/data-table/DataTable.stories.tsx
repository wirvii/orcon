// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { RelativeTime } from '../relative-time/RelativeTime';
import { StatusBadge } from '../status-badge/StatusBadge';
import type { StatusKey } from '../status-badge/status-map';

import { DataTable, type DataColumn, type SortState } from './DataTable';

interface Execution {
  id: string;
  flow: string;
  status: StatusKey;
  started: string;
}

const executions: Execution[] = [
  { id: '4821', flow: 'Nightly sync', status: 'SUCCEEDED', started: '2026-03-14T14:55:00.000Z' },
  { id: '4820', flow: 'Order import', status: 'FAILED', started: '2026-03-14T14:20:00.000Z' },
  { id: '4819', flow: 'Welcome email', status: 'RUNNING', started: '2026-03-14T14:05:00.000Z' },
  { id: '4818', flow: 'Refund review', status: 'UNCERTAIN', started: '2026-03-14T13:40:00.000Z' },
];

const now = '2026-03-14T15:00:00.000Z';

const columns: DataColumn<Execution>[] = [
  { id: 'id', header: 'Execution', cell: (row) => row.id, sortable: true },
  { id: 'flow', header: 'Flow', cell: (row) => row.flow, sortable: true },
  { id: 'status', header: 'Status', cell: (row) => <StatusBadge status={row.status} /> },
  { id: 'started', header: 'Started', cell: (row) => <RelativeTime now={now} value={row.started} />, align: 'end' },
];

const meta = {
  title: 'Components/DataTable',
  component: DataTable<Execution>,
  args: { label: 'Executions', columns, rows: executions, getRowId: (row: Execution) => row.id, className: 'max-w-screen-md' },
} satisfies Meta<typeof DataTable<Execution>>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole('region', { name: 'Executions' })).toBeInTheDocument();
    await expect(canvas.getAllByRole('row')).toHaveLength(1 + executions.length);
  },
};

/** The caller sorts (usually on the server); the header reports the order with aria-sort and an icon. */
export const Sorted: Story = {
  render: function Render(args) {
    const [sort, setSort] = useState<SortState>({ columnId: 'flow', direction: 'ascending' });
    const sorted = [...args.rows].sort((a, b) => (sort.direction === 'ascending' ? 1 : -1) * a.flow.localeCompare(b.flow));
    return <DataTable {...args} rows={sorted} sort={sort} onSortChange={setSort} />;
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const header = canvas.getByRole('columnheader', { name: /Flow/ });
    await expect(header).toHaveAttribute('aria-sort', 'ascending');
    await userEvent.click(within(header).getByRole('button'));
    await expect(header).toHaveAttribute('aria-sort', 'descending');
    await expect(canvas.getAllByRole('row')[1]).toHaveTextContent('Welcome email');
  },
};
export const Loading: Story = { args: { status: 'loading' } };
export const Empty: Story = { args: { rows: [], emptyMessage: 'No executions yet. Run a flow to see it here.' } };
export const Error: Story = { args: { status: 'error', onRetry: () => undefined } };
export const StickyHeader: Story = {
  args: { stickyHeader: true, className: 'max-h-panel-width max-w-screen-md', rows: [...executions, ...executions.map((row) => ({ ...row, id: `${row.id}-b` }))] },
};

const manyExecutions: Execution[] = Array.from({ length: 5000 }, (_, index) => ({
  id: String(9000 - index),
  flow: `Flow ${String(index + 1)}`,
  status: index % 9 === 0 ? 'FAILED' : 'SUCCEEDED',
  started: '2026-03-14T14:55:00.000Z',
}));

/** For a list that can pass 100 rows: only the rows near the visible part are rendered, the scrollbar stays true. */
export const Virtual: Story = {
  args: { virtual: true, rows: manyExecutions, className: 'max-h-panel-width max-w-screen-md' },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole('table')).toHaveAttribute('aria-rowcount', String(manyExecutions.length + 1));
    await expect(canvas.getAllByRole('row').length).toBeLessThan(80);
  },
};

const stateArgs: Record<string, Partial<React.ComponentProps<typeof DataTable<Execution>>>> = {
  default: {},
  sorted: { sort: { columnId: 'flow', direction: 'ascending' }, onSortChange: () => undefined },
  loading: { status: 'loading' },
  empty: { rows: [], emptyMessage: 'No executions yet.' },
  error: { status: 'error', onRetry: () => undefined },
  'sticky-header': { stickyHeader: true, className: 'max-h-panel-width' },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="DataTable states"
      columns={['table']}
      rows={Object.entries(stateArgs).map(([state, args]) => ({
        label: state,
        cells: [
          <div key={state} className="w-full max-w-screen-md">
            <DataTable<Execution> label={`Executions, ${state}`} columns={columns} rows={executions} getRowId={(row) => row.id} {...args} />
          </div>,
        ],
      }))}
    />
  ),
};
