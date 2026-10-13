// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';
import type { StatusKey } from '../../components/status-badge/status-map';

import { NodeStatusBadge } from './NodeStatusBadge';

const meta = {
  title: 'Canvas/NodeStatusBadge',
  component: NodeStatusBadge,
  args: { status: 'SUCCEEDED', corner: false },
  decorators: [
    (Story) => (
      <div className="bg-canvas inline-block rounded-lg p-6">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof NodeStatusBadge>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Running: Story = { args: { status: 'RUNNING' } };
export const Succeeded: Story = { args: { status: 'SUCCEEDED' } };
export const SucceededWithErrors: Story = { args: { status: 'SUCCEEDED_WITH_ERRORS' } };
export const Failed: Story = { args: { status: 'FAILED' } };
export const Uncertain: Story = { args: { status: 'UNCERTAIN' } };
export const Canceled: Story = { args: { status: 'CANCELED' } };
export const NotRun: Story = { args: { status: 'NOT_RUN' } };

const statuses: StatusKey[] = ['RUNNING', 'SUCCEEDED', 'SUCCEEDED_WITH_ERRORS', 'FAILED', 'UNCERTAIN', 'WAITING', 'SUSPENDED', 'CANCELED', 'NOT_RUN'];

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Node status badge"
      columns={statuses}
      rows={[{ label: '22 px badge', cells: statuses.map((status) => <div key={status} className="bg-canvas rounded-lg p-3"><NodeStatusBadge status={status} corner={false} /></div>) }]}
    />
  ),
};
