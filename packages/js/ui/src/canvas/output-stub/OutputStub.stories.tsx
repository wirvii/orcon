// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { OutputStub } from './OutputStub';

const meta = {
  title: 'Canvas/OutputStub',
  component: OutputStub,
  args: { label: 'Add node after Merge', top: 0 },
  decorators: [
    (Story) => (
      <div className="bg-canvas relative inline-block rounded-lg py-10 ps-10 pe-16">
        <div className="relative h-0 w-4">
          <Story />
        </div>
      </div>
    ),
  ],
} satisfies Meta<typeof OutputStub>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Hover: Story = { args: { className: 'pseudo-hover' } };
export const FocusVisible: Story = { args: { className: 'pseudo-focus-visible' } };

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Unconnected output"
      columns={['default', 'hover', 'focus-visible']}
      rows={[
        {
          label: 'stub',
          cells: ['', 'pseudo-hover', 'pseudo-focus-visible'].map((className) => (
            <div key={className} className="bg-canvas relative rounded-lg py-8 ps-8 pe-16">
              <div className="relative h-0 w-4">
                <OutputStub label="Add node" top={0} className={className} />
              </div>
            </div>
          )),
        },
      ]}
    />
  ),
};
