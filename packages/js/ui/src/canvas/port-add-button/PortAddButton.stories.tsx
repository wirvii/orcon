// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { PortAddButton } from './PortAddButton';
import type { states } from './PortAddButton.meta';

const meta = {
  title: 'Canvas/PortAddButton',
  component: PortAddButton,
  args: { id: 'main', label: 'Add node after HTTP Request', top: 0 },
  decorators: [
    (Story) => (
      <div className="bg-canvas relative inline-block rounded-lg px-16 py-10">
        <div className="relative h-0 w-6">
          <Story />
        </div>
      </div>
    ),
  ],
} satisfies Meta<typeof PortAddButton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Hover: Story = { args: { className: 'pseudo-hover' } };
export const FocusVisible: Story = { args: { className: 'pseudo-focus-visible' } };
export const Selected: Story = { args: { selected: true } };

const cells: Record<(typeof states)[number], Parameters<typeof PortAddButton>[0]> = {
  default: { id: 'a', label: 'Add node', top: 0 },
  hover: { id: 'b', label: 'Add node', top: 0, className: 'pseudo-hover' },
  'focus-visible': { id: 'c', label: 'Add node', top: 0, className: 'pseudo-focus-visible' },
  selected: { id: 'd', label: 'Add node', top: 0, selected: true },
};

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Output port"
      columns={Object.keys(cells)}
      rows={[
        {
          label: 'output',
          cells: (Object.keys(cells) as (keyof typeof cells)[]).map((key) => (
            <div key={key} className="bg-canvas relative rounded-lg px-10 py-8">
              <div className="relative h-0 w-4">
                <PortAddButton {...cells[key]} />
              </div>
            </div>
          )),
        },
      ]}
    />
  ),
};
