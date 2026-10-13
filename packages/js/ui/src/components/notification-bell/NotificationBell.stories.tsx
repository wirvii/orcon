// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { NotificationBell } from './NotificationBell';
import { states } from './NotificationBell.meta';

const meta = {
  title: 'Components/NotificationBell',
  component: NotificationBell,
  args: { count: 0 },
  decorators: [(Story) => <div className="p-2"><Story /></div>],
} satisfies Meta<typeof NotificationBell>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('button', { name: 'Nothing needs your attention' })).toBeInTheDocument();
  },
};
export const Hover: Story = { args: { count: 2 }, parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { args: { count: 2 }, parameters: { pseudo: { focusVisible: true } } };
export const WithCount: Story = {
  args: { count: 3 },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('button', { name: '3 items need your attention' })).toBeInTheDocument();
  },
};
export const LargeCount: Story = { args: { count: 142 } };

const stateProps: Record<(typeof states)[number], { count: number; className?: string }> = {
  default: { count: 0 },
  hover: { count: 2, className: 'pseudo-hover' },
  'focus-visible': { count: 2, className: 'pseudo-focus-visible' },
  'with-count': { count: 3 },
  'large-count': { count: 142 },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="NotificationBell states"
      columns={[...states]}
      rows={[{ label: 'bell', cells: states.map((state) => <div key={state} className="p-1"><NotificationBell {...stateProps[state]} /></div>) }]}
    />
  ),
};
