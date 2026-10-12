// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { CanvasStage } from '../stories/CanvasStage';
import { MatrixTable } from '../stories/MatrixTable';

import { EdgeCount } from './EdgeCount';

const meta = {
  title: 'Canvas/EdgeCount',
  component: EdgeCount,
  args: { count: 12 },
  decorators: [(Story) => <CanvasStage><Story /></CanvasStage>],
} satisfies Meta<typeof EdgeCount>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('12 items')).toBeVisible();
  },
};
export const OneItem: Story = {
  args: { count: 1 },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('1 item')).toBeVisible();
  },
};
export const LoopBack: Story = { args: { count: 3, loopBack: true } };

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Item count chip"
      columns={['forward', 'loop back']}
      rows={[0, 1, 12, 1200].map((count) => ({
        label: String(count),
        cells: [false, true].map((loopBack) => (
          <CanvasStage key={String(loopBack)}>
            <EdgeCount count={count} loopBack={loopBack} />
          </CanvasStage>
        )),
      }))}
    />
  ),
};
