// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { Separator } from './Separator';

const meta = {
  title: 'Components/Separator',
  component: Separator,
} satisfies Meta<typeof Separator>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  decorators: [
    (Story) => (
      <div className="bg-surface flex max-w-screen-sm flex-col gap-3 p-4">
        Above
        <Story />
        Below
      </div>
    ),
  ],
};

export const Vertical: Story = {
  args: { orientation: 'vertical' },
  decorators: [
    (Story) => (
      <div className="bg-surface flex h-12 items-center gap-3 p-4">
        Left
        <Story />
        Right
      </div>
    ),
  ],
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Separator orientations"
      columns={['default', 'vertical']}
      rows={[
        {
          label: 'separator',
          cells: [
            <div key="h" className="bg-surface w-panel-width p-2">
              <Separator />
            </div>,
            <div key="v" className="bg-surface flex h-12 items-center p-2">
              <Separator orientation="vertical" />
            </div>,
          ],
        },
      ]}
    />
  ),
};
