// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../stories/MatrixTable';

import { CanvasOutline } from './CanvasOutline';
import { exampleGraph } from './fixtures';

const meta = {
  title: 'Canvas/CanvasOutline',
  component: CanvasOutline,
  args: { graph: exampleGraph },
} satisfies Meta<typeof CanvasOutline>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByText('6 nodes and 6 connections')).toBeVisible();
    await expect(view.getByText('Output true goes to Charge card, input main, 9 items')).toBeVisible();
  },
};
export const Editable: Story = {
  args: { editable: true },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('button', { name: 'Delete Check stock' })).toBeVisible();
  },
};
export const Empty: Story = { args: { graph: { nodes: [], edges: [] } } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Flow outline"
      columns={['view', 'editable']}
      rows={[
        {
          label: 'example flow',
          cells: [<CanvasOutline key="v" graph={exampleGraph} headingLevel={3} />, <CanvasOutline key="e" graph={exampleGraph} editable headingLevel={3} />],
        },
      ]}
    />
  ),
};
