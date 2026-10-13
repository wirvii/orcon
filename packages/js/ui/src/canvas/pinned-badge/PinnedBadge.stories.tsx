// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { PinnedBadge } from './PinnedBadge';

const meta = {
  title: 'Canvas/PinnedBadge',
  component: PinnedBadge,
  args: { corner: false },
  decorators: [
    (Story) => (
      <div className="bg-canvas inline-block rounded-lg p-6">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof PinnedBadge>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Pinned badge"
      columns={['default']}
      rows={[{ label: '22 px badge', cells: [<div key="a" className="bg-canvas rounded-lg p-3"><PinnedBadge corner={false} /></div>] }]}
    />
  ),
};
