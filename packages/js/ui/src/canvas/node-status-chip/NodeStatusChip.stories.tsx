// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { NodeStatusChip, type NodeStatusChipProps } from './NodeStatusChip';

const meta = {
  title: 'Canvas/NodeStatusChip',
  component: NodeStatusChip,
  args: { status: 'RUNNING' },
} satisfies Meta<typeof NodeStatusChip>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('Running')).toBeVisible();
  },
};
export const Running: Story = { args: { status: 'RUNNING' } };
export const Failed: Story = { args: { status: 'FAILED', code: '401' } };
export const Uncertain: Story = { args: { status: 'UNCERTAIN' } };
export const Waiting: Story = { args: { status: 'SUSPENDED' } };
export const Canceled: Story = { args: { status: 'CANCELED' } };
export const NotRun: Story = { args: { status: 'NOT_RUN' } };
export const Pinned: Story = { args: { status: undefined, kind: 'pinned' } };
export const Disabled: Story = { args: { status: undefined, kind: 'disabled' } };
export const NotSupported: Story = { args: { status: undefined, kind: 'not-supported' } };
export const Compare: Story = { args: { status: undefined, kind: 'changed' } };

const chips: [string, NodeStatusChipProps][] = [
  ['Running', { status: 'RUNNING' }],
  ['Succeeded with errors', { status: 'SUCCEEDED_WITH_ERRORS' }],
  ['Failed', { status: 'FAILED', code: '401' }],
  ['Uncertain', { status: 'UNCERTAIN' }],
  ['Uncertain items', { kind: 'uncertain-items', count: 3 }],
  ['Waiting', { status: 'WAITING' }],
  ['Needs action', { status: 'SUSPENDED' }],
  ['Canceled', { status: 'CANCELED' }],
  ['Not run', { status: 'NOT_RUN' }],
  ['Pinned', { kind: 'pinned' }],
  ['Disabled', { kind: 'disabled' }],
  ['Not supported', { kind: 'not-supported' }],
  ['Added', { kind: 'added' }],
  ['Changed', { kind: 'changed' }],
  ['Removed', { kind: 'removed' }],
];

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Node status chips"
      columns={['chip']}
      rows={chips.map(([label, props]) => ({ label, cells: [<div key={label} className="bg-canvas rounded-lg p-3"><NodeStatusChip {...props} /></div>] }))}
    />
  ),
};
