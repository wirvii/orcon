// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../stories/MatrixTable';
import { statusKeys } from '../components/status-badge/status-map';

import { NodeStatus } from './NodeStatus';

const meta = {
  title: 'Canvas/NodeStatus',
  component: NodeStatus,
  args: { status: 'QUEUED' },
} satisfies Meta<typeof NodeStatus>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = { args: { status: 'QUEUED' } };
export const Running: Story = { args: { status: 'RUNNING' } };
export const Succeeded: Story = { args: { status: 'SUCCEEDED' } };
export const SucceededWithErrors: Story = { args: { status: 'SUCCEEDED_WITH_ERRORS' } };
export const Failed: Story = { args: { status: 'FAILED' } };
export const Uncertain: Story = {
  args: { status: 'UNCERTAIN' },
  play: async ({ canvasElement }) => {
    // UNCERTAIN is never named or drawn as a failure or a success.
    const status = within(canvasElement).getByText('Uncertain').closest('[aria-describedby]');
    await expect(status).toHaveAccessibleDescription(
      'The external system may or may not have applied this operation. Review it before continuing.',
    );
  },
};
export const NotRun: Story = { args: { status: 'NOT_RUN' } };
export const WithUncertainItems: Story = {
  args: { status: 'SUCCEEDED_WITH_ERRORS', uncertainCount: 3 },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('3 uncertain items')).toBeVisible();
  },
};

/** Every state a node can show, as icon + text. */
export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Node status"
      columns={['icon and text']}
      rows={statusKeys.map((key) => ({ label: key, cells: [<NodeStatus key={key} status={key} />] }))}
    />
  ),
};
