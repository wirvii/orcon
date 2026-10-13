// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { manyItems, mixedItems, orderItems } from '../fixtures';

import { DataTableView } from './DataTableView';

const meta = {
  title: 'Editor/DataTableView',
  component: DataTableView,
  args: { items: orderItems },
  decorators: [
    (Story) => (
      <div className="bg-surface border-border h-canvas-palette-w max-w-screen-md overflow-hidden rounded-lg border">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof DataTableView>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('columnheader', { name: '#' })).toBeVisible();
    // null is dimmed, nested data is collapsed and expandable.
    await expect(view.getAllByText('null').length).toBeGreaterThan(0);
    const toggle = view.getAllByRole('button', { name: /, shipping$/ })[0]!;
    await expect(toggle).toHaveTextContent('{…} 3 keys');
    await userEvent.click(toggle);
    await expect(within(canvasElement).getByText(/"city": "Neiva"/)).toBeVisible();
  },
};
export const Nested: Story = { args: { items: orderItems } };
export const Protected: Story = {
  args: { items: mixedItems },
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getAllByText('Hidden — you need permission to view execution data').length).toBeGreaterThan(0);
    await expect(view.getAllByText('Secret value').length).toBeGreaterThan(0);
    await expect(view.getAllByText('Hidden in history').length).toBeGreaterThan(0);
  },
};
export const Binary: Story = { args: { items: mixedItems.map(({ invoice, id }) => ({ id, invoice })) } };
export const Empty: Story = { args: { items: [] } };
export const Virtualized: Story = {
  args: { items: manyItems(1000) },
  play: async ({ canvasElement }) => {
    // Only the rows in view are in the page.
    await expect(canvasElement.querySelectorAll('tbody tr[data-item-row]').length).toBeLessThan(60);
  },
};

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Data table"
      columns={['view']}
      rows={[
        { label: 'items', cells: [<div key="a" className="bg-surface border-border max-w-screen-md overflow-hidden rounded-lg border"><DataTableView items={orderItems} /></div>] },
        { label: 'protected and files', cells: [<div key="b" className="bg-surface border-border max-w-screen-md overflow-hidden rounded-lg border"><DataTableView items={mixedItems} /></div>] },
        { label: 'empty', cells: [<div key="c" className="bg-surface border-border max-w-screen-md overflow-hidden rounded-lg border"><DataTableView items={[]} /></div>] },
      ]}
    />
  ),
};
