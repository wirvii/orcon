// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { EdgeActions } from './EdgeActions';

const meta = {
  title: 'Canvas/EdgeActions',
  component: EdgeActions,
  args: { source: 'HTTP Request', target: 'Slack', tabbable: true },
  decorators: [
    (Story) => (
      <div className="bg-canvas inline-block rounded-lg p-6">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof EdgeActions>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('button', { name: 'Add node here' })).toBeVisible();
    await expect(view.getByRole('button', { name: 'Delete connection' })).toBeVisible();
  },
};
export const Hover: Story = { args: { className: 'pseudo-hover' } };
export const FocusVisible: Story = { args: { className: 'pseudo-focus-visible' } };

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Connection actions"
      columns={['default']}
      rows={[{ label: 'bar', cells: [<div key="a" className="bg-canvas rounded-lg p-4"><EdgeActions source="HTTP Request" target="Slack" tabbable /></div>] }]}
    />
  ),
};
