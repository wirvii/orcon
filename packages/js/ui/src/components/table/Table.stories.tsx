// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { StatusBadge } from '../status-badge/StatusBadge';

import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from './Table';

const rows = [
  { id: '4821', flow: 'Nightly sync', status: 'SUCCEEDED' },
  { id: '4820', flow: 'Order import', status: 'FAILED' },
  { id: '4819', flow: 'Welcome email', status: 'RUNNING' },
] as const;

function Demo({ selected, sticky, hover }: { selected?: string; sticky?: boolean; hover?: boolean }) {
  return (
    <div className="bg-surface max-h-node-width max-w-screen-sm overflow-auto rounded-md border border-border" role="region" aria-label="Executions table" tabIndex={0}>
      <Table>
        <TableCaption visuallyHidden>Recent executions</TableCaption>
        <TableHeader>
          <TableRow header>
            <TableHead sticky={sticky}>Execution</TableHead>
            <TableHead sticky={sticky}>Flow</TableHead>
            <TableHead sticky={sticky}>Status</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row, index) => (
            <TableRow key={row.id} selected={row.id === selected} className={hover && index === 1 ? 'pseudo-hover' : undefined}>
              <TableCell>{row.id}</TableCell>
              <TableCell>{row.flow}</TableCell>
              <TableCell>
                <StatusBadge status={row.status} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

const meta = {
  title: 'Components/Table',
  component: Table,
  render: () => <Demo />,
} satisfies Meta<typeof Table>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getAllByRole('columnheader')).toHaveLength(3);
    // Rows are as tall as the density says: the density tests measure them.
    await expect(canvasElement.querySelectorAll('[data-row]')).toHaveLength(3);
  },
};
export const Hover: Story = { render: () => <Demo hover />, parameters: { pseudo: { hover: false } } };
export const Selected: Story = {
  render: () => <Demo selected="4820" />,
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getAllByRole('row')[2]).toHaveAttribute('data-state', 'selected');
  },
};
export const StickyHeader: Story = { render: () => <Demo sticky /> };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Table states"
      columns={['table']}
      rows={[
        { label: 'default', cells: [<Demo key="d" />] },
        { label: 'selected', cells: [<Demo key="s" selected="4820" />] },
        { label: 'sticky header', cells: [<Demo key="h" sticky />] },
      ]}
    />
  ),
};
