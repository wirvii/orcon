// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { NodeToolbar, type NodeToolbarProps } from './NodeToolbar';

const meta = {
  title: 'Canvas/NodeToolbar',
  component: NodeToolbar,
  args: { name: 'HTTP Request' },
  decorators: [
    (Story) => (
      <div className="bg-canvas relative inline-block rounded-lg px-10 pt-16 pb-6">
        <div className="relative h-0 w-canvas-label-width">
          <Story />
        </div>
      </div>
    ),
  ],
} satisfies Meta<typeof NodeToolbar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const toolbar = within(canvasElement).getByRole('toolbar', { name: 'Actions for HTTP Request' });
    await expect(within(toolbar).getByRole('button', { name: 'Run step' })).toBeVisible();
    await expect(within(toolbar).getByRole('button', { name: 'Disable node' })).toBeVisible();
    await expect(within(toolbar).getByRole('button', { name: 'Delete node' })).toBeVisible();
    await expect(within(toolbar).getByRole('button', { name: 'More actions' })).toBeVisible();
  },
};
export const Trigger: Story = { args: { trigger: true } };
export const DisabledNode: Story = { args: { disabled: true } };
export const Hover: Story = { args: { className: 'pseudo-hover' } };
export const FocusVisible: Story = { args: { className: 'pseudo-focus-visible' } };
export const Unsupported: Story = { args: { canRun: false, disabled: true } };

const variants: [string, NodeToolbarProps][] = [
  ['action', { name: 'HTTP Request' }],
  ['trigger', { name: 'Webhook', trigger: true }],
  ['disabled node', { name: 'Merge', disabled: true }],
  ['unsupported', { name: 'Custom code', canRun: false, disabled: true }],
];

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Node toolbar"
      columns={['toolbar']}
      rows={variants.map(([label, props]) => ({
        label,
        cells: [
          <div key={label} className="bg-canvas relative rounded-lg px-10 pt-16 pb-2">
            <div className="relative h-0 w-canvas-label-width">
              <NodeToolbar {...props} />
            </div>
          </div>,
        ],
      }))}
    />
  ),
};
