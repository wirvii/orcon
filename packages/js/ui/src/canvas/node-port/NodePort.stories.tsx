// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { NodePort } from './NodePort';

const meta = {
  title: 'Canvas/NodePort',
  component: NodePort,
  args: { side: 'input', id: 'main', top: 0 },
  decorators: [
    (Story) => (
      <div className="bg-canvas relative inline-block rounded-lg px-16 py-10">
        <div className="relative h-0 w-4">
          <Story />
        </div>
      </div>
    ),
  ],
} satisfies Meta<typeof NodePort>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Input: Story = { args: { side: 'input' } };
export const Output: Story = { args: { side: 'output', label: 'Add node' } };
export const Selected: Story = { args: { selected: true } };
export const ConnectingValid: Story = { args: { className: 'valid connectingto' } };

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Ports"
      columns={['default', 'selected', 'connecting-valid']}
      rows={(['input', 'output'] as const).map((side) => ({
        label: side,
        cells: [{}, { selected: true }, { className: 'valid connectingto' }].map((extra, index) => (
          <div key={index} className="bg-canvas relative rounded-lg px-10 py-8">
            <div className="relative h-0 w-4">
              <NodePort side={side} id="main" top={0} label="Add node" {...extra} />
            </div>
          </div>
        )),
      }))}
    />
  ),
};
