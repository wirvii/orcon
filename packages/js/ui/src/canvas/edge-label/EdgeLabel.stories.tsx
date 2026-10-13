// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { EdgeLabel, type EdgeLabelProps } from './EdgeLabel';

const meta = {
  title: 'Canvas/EdgeLabel',
  component: EdgeLabel,
  args: { items: 3, state: 'succeeded' },
  decorators: [
    (Story) => (
      <div className="bg-canvas inline-block rounded-lg p-6">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof EdgeLabel>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('3 items')).toBeVisible();
  },
};
export const Succeeded: Story = { args: { state: 'succeeded', items: 1 } };
export const Running: Story = { args: { state: 'running', items: 3 } };
export const Failed: Story = { args: { state: 'failed', items: 0 } };
export const Uncertain: Story = { args: { state: 'uncertain' } };
export const Pinned: Story = { args: { state: 'succeeded', items: 3, pinned: true } };
export const LoopBack: Story = { args: { state: 'succeeded', items: 2, loopBack: true } };

const labels: [string, EdgeLabelProps][] = [
  ['succeeded, 1 item', { state: 'succeeded', items: 1 }],
  ['succeeded, 3 items', { state: 'succeeded', items: 3 }],
  ['running', { state: 'running', items: 3 }],
  ['failed, 0 items', { state: 'failed', items: 0 }],
  ['uncertain', { state: 'uncertain' }],
  ['pinned', { state: 'succeeded', items: 3, pinned: true }],
  ['loop back', { state: 'succeeded', items: 2, loopBack: true }],
  ['not run', { state: 'not-run', items: 0 }],
];

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Connection labels"
      columns={['label']}
      rows={labels.map(([label, props]) => ({ label, cells: [<div key={label} className="bg-canvas rounded-lg p-3"><EdgeLabel {...props} /></div>] }))}
    />
  ),
};
