// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { CanvasStage } from '../stories/CanvasStage';
import { MatrixTable } from '../stories/MatrixTable';

import { Port } from './Port';

const meta = {
  title: 'Canvas/Port',
  component: Port,
  args: { side: 'output', id: 'main' },
  decorators: [
    (Story) => (
      <CanvasStage>
        <div className="bg-surface-raised border-canvas-node-border relative h-6 w-16 rounded-md border">
          <Story />
        </div>
      </CanvasStage>
    ),
  ],
} satisfies Meta<typeof Port>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Input: Story = { args: { side: 'input' } };
export const Output: Story = { args: { side: 'output' } };
export const Selected: Story = { args: { selected: true } };

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Port: inputs on the left edge, outputs on the right edge"
      columns={['default', 'selected']}
      rows={(['input', 'output'] as const).map((side) => ({
        label: side,
        cells: [false, true].map((selected) => (
          <CanvasStage key={String(selected)}>
            <div className="bg-surface-raised border-canvas-node-border relative h-6 w-16 rounded-md border">
              <Port side={side} id="main" selected={selected} />
            </div>
          </CanvasStage>
        )),
      }))}
    />
  ),
};
