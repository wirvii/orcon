// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { EdgePreview } from '../stories/CanvasStage';
import { MatrixTable } from '../stories/MatrixTable';


const meta = {
  title: 'Canvas/Edge',
  component: EdgePreview,
} satisfies Meta<typeof EdgePreview>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Active: Story = { args: { data: { active: true } } };
export const Selected: Story = { args: { selected: true } };
export const WithCount: Story = { args: { data: { items: 12, active: true } } };
export const LoopBack: Story = { args: { data: { items: 3 }, loopBack: true } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Connections"
      columns={['preview']}
      rows={[
        { label: 'default', cells: [<EdgePreview key="d" />] },
        { label: 'active with count', cells: [<EdgePreview key="a" data={{ items: 12, active: true }} />] },
        { label: 'selected', cells: [<EdgePreview key="s" selected />] },
        { label: 'loop back', cells: [<EdgePreview key="l" data={{ items: 3 }} loopBack />] },
      ]}
    />
  ),
};
