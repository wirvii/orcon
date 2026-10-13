// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, waitFor, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { mixedItems, orderItems } from '../fixtures';

import { JsonTreeView } from './JsonTreeView';
import { must } from '../../lib/must';

const meta = {
  title: 'Editor/JsonTreeView',
  component: JsonTreeView,
  args: { value: orderItems },
  decorators: [
    (Story) => (
      <div className="bg-surface border-border max-w-screen-md rounded-lg border">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof JsonTreeView>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const tree = within(canvasElement).getByRole('tree', { name: 'Data as JSON' });
    // One tab stop; arrows move between rows and collapse or expand.
    const first = must(within(tree).getAllByRole('treeitem')[0]);
    first.focus();
    await userEvent.keyboard('{ArrowDown}');
    await waitFor(() => expect(within(tree).getAllByRole('treeitem')[1]).toHaveFocus());
    await userEvent.keyboard('{ArrowLeft}');
    await waitFor(() => expect(within(tree).getAllByRole('treeitem')[1]).toHaveAttribute('aria-expanded', 'false'));
  },
};
export const Collapsed: Story = { args: { defaultExpandedDepth: 0 } };
export const Protected: Story = { args: { value: mixedItems } };
export const FocusVisible: Story = {
  play: ({ canvasElement }) => {
    must(within(canvasElement).getAllByRole('treeitem')[1]).focus();
  },
};

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="JSON tree"
      columns={['view']}
      rows={[
        { label: 'items', cells: [<div key="a" className="bg-surface border-border max-w-screen-md rounded-lg border"><JsonTreeView value={orderItems.slice(0, 1)} /></div>] },
        { label: 'collapsed', cells: [<div key="b" className="bg-surface border-border max-w-screen-md rounded-lg border"><JsonTreeView value={orderItems} defaultExpandedDepth={1} /></div>] },
        { label: 'protected', cells: [<div key="c" className="bg-surface border-border max-w-screen-md rounded-lg border"><JsonTreeView value={mixedItems.slice(0, 1)} /></div>] },
      ]}
    />
  ),
};
