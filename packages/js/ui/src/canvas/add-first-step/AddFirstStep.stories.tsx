// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { AddFirstStep } from './AddFirstStep';

const meta = {
  title: 'Canvas/AddFirstStep',
  component: AddFirstStep,
  decorators: [
    (Story) => (
      <div className="bg-canvas inline-block rounded-lg px-16 py-12">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof AddFirstStep>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('button', { name: 'Add first step' })).toBeVisible();
  },
};
export const Hover: Story = { args: { className: 'pseudo-hover' } };
export const FocusVisible: Story = { args: { className: 'pseudo-focus-visible' } };

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Add first step"
      columns={['default']}
      rows={[{ label: 'empty canvas', cells: [<div key="a" className="bg-canvas rounded-lg px-10 py-8"><AddFirstStep /></div>] }]}
    />
  ),
};
