// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { Skeleton } from './Skeleton';

const meta = {
  title: 'Components/Skeleton',
  component: Skeleton,
  args: { className: 'h-12 w-panel-width' },
} satisfies Meta<typeof Skeleton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Text: Story = { args: { shape: 'text', className: 'w-panel-width' } };
export const Circle: Story = { args: { shape: 'circle', className: 'size-control' } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Skeleton shapes"
      columns={['default', 'text', 'circle']}
      rows={[
        {
          label: 'skeleton',
          cells: [
            <Skeleton key="b" className="h-10 w-16" />,
            <Skeleton key="t" shape="text" className="w-16" />,
            <Skeleton key="c" shape="circle" className="size-control" />,
          ],
        },
      ]}
    />
  ),
};
