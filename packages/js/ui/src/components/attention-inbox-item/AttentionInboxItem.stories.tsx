// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { AttentionInboxItem } from './AttentionInboxItem';
import { states } from './AttentionInboxItem.meta';

const now = '2026-03-14T15:00:00.000Z';

const meta = {
  title: 'Components/AttentionInboxItem',
  component: AttentionInboxItem,
  args: {
    title: 'Approve the refund of order 48213',
    href: '#approval',
    flowName: 'Refund review',
    createdAt: '2026-03-14T13:40:00.000Z',
    now,
  },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
        <div className="bg-surface max-w-screen-md rounded-md border border-border">
          <Story />
        </div>
      ),
  ],
} satisfies Meta<typeof AttentionInboxItem>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole('article', { name: 'Approve the refund of order 48213' })).toBeInTheDocument();
    await expect(canvas.getByText('1 hour ago')).toBeInTheDocument();
  },
};
export const Selected: Story = {
  args: { selected: true, onSelectedChange: () => undefined },
  render: (args) => {
    const [selected, setSelected] = useState(true);
    return <AttentionInboxItem {...args} selected={selected} onSelectedChange={setSelected} />;
  },
  play: async ({ canvasElement }) => {
    const box = within(canvasElement).getByRole('checkbox', { name: 'Select Approve the refund of order 48213' });
    await expect(box).toBeChecked();
    await userEvent.click(box);
    await expect(box).not.toBeChecked();
  },
};
export const Acknowledged: Story = { args: { acknowledgedBy: 'Ada Lovelace' } };
export const Escalated: Story = { args: { escalationLevel: 2 } };
export const Overdue: Story = { args: { overdue: true, escalationLevel: 1 } };

const stateArgs: Record<(typeof states)[number], Partial<React.ComponentProps<typeof AttentionInboxItem>>> = {
  default: {},
  selected: { selected: true, onSelectedChange: () => undefined },
  acknowledged: { acknowledgedBy: 'Ada Lovelace' },
  escalated: { escalationLevel: 2 },
  overdue: { overdue: true, escalationLevel: 1 },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="AttentionInboxItem states"
      columns={['item']}
      rows={states.map((state) => ({
        label: state,
        cells: [
          <div key={state} className="bg-surface w-full max-w-screen-md rounded-md border border-border">
            <AttentionInboxItem
              title="Approve the refund of order 48213"
              href="#approval"
              flowName="Refund review"
              createdAt="2026-03-14T13:40:00.000Z"
              now={now}
              {...stateArgs[state]}
            />
          </div>,
        ],
      }))}
    />
  ),
};
