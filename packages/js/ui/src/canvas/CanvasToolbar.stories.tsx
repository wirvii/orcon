// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../stories/MatrixTable';

import { CanvasToolbar } from './CanvasToolbar';

const meta = {
  title: 'Canvas/CanvasToolbar',
  component: CanvasToolbar,
  args: { editable: true },
} satisfies Meta<typeof CanvasToolbar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const toolbar = within(canvasElement).getByRole('toolbar', { name: 'Canvas tools' });
    // Every graph operation has a button.
    for (const name of ['Add node', 'Connect nodes', 'Delete selected', 'Undo', 'Redo']) {
      await expect(within(toolbar).getByRole('button', { name })).toBeInTheDocument();
    }
  },
};
export const CanUndo: Story = { args: { canUndo: true, canRedo: true, canDelete: true } };
export const ReadOnly: Story = {
  args: { editable: false },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).queryByRole('button', { name: 'Add node' })).toBeNull();
    await expect(within(canvasElement).getByRole('button', { name: 'Zoom in' })).toBeVisible();
  },
};
export const OutlineOpen: Story = { args: { outlineOpen: true } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Canvas toolbar"
      columns={['toolbar']}
      rows={[
        { label: 'editing', cells: [<CanvasToolbar key="a" editable canUndo canDelete />] },
        { label: 'nothing to undo', cells: [<CanvasToolbar key="b" editable />] },
        { label: 'view only', cells: [<CanvasToolbar key="c" editable={false} outlineOpen />] },
      ]}
    />
  ),
};
