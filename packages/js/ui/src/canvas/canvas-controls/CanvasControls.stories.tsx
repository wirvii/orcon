// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { CanvasControls } from './CanvasControls';

const meta = {
  title: 'Canvas/CanvasControls',
  component: CanvasControls,
  args: { zoom: 1 },
  decorators: [
    (Story) => (
      <div className="bg-canvas inline-block rounded-lg p-6">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof CanvasControls>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('button', { name: 'Zoom in' })).toBeVisible();
    // Reset to 100% shows only when the zoom is not 100%.
    await expect(view.queryByRole('button', { name: 'Reset zoom to 100%' })).toBeNull();
  },
};
export const Zoomed: Story = {
  args: { zoom: 0.5 },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('button', { name: 'Reset zoom to 100%' })).toBeVisible();
  },
};
export const WithTidyUp: Story = { args: { onTidyUp: () => undefined } };
export const Hover: Story = { args: { className: 'pseudo-hover' } };
export const FocusVisible: Story = { args: { className: 'pseudo-focus-visible' } };

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Zoom controls"
      columns={['at 100%', 'zoomed', 'with tidy up']}
      rows={[
        {
          label: 'controls',
          cells: [
            <div key="a" className="bg-canvas rounded-lg p-4"><CanvasControls zoom={1} /></div>,
            <div key="b" className="bg-canvas rounded-lg p-4"><CanvasControls zoom={0.5} /></div>,
            <div key="c" className="bg-canvas rounded-lg p-4"><CanvasControls zoom={1} onTidyUp={() => undefined} /></div>,
          ],
        },
      ]}
    />
  ),
};
