// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { ExpressionChip } from './ExpressionChip';

const meta = {
  title: 'Editor/ExpressionChip',
  component: ExpressionChip,
  args: { kind: 'path', label: 'customer.phone' },
} satisfies Meta<typeof ExpressionChip>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('customer.phone')).toBeVisible();
  },
};
export const Node: Story = { args: { kind: 'node', node: 'Webhook', label: 'body.id' } };
export const Raw: Story = { args: { kind: 'raw', label: '$json.total * 2' } };
export const Removable: Story = {
  args: { onRemove: fn() },
  play: async ({ canvasElement, args }) => {
    await userEvent.click(within(canvasElement).getByRole('button', { name: 'Remove customer.phone' }));
    await expect(args.onRemove).toHaveBeenCalled();
  },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Expression chips"
      columns={['chip', 'with remove']}
      rows={[
        { label: 'field', cells: [<ExpressionChip key="a" kind="path" label="customer.phone" />, <ExpressionChip key="b" kind="path" label="customer.phone" onRemove={() => undefined} />] },
        { label: 'other step', cells: [<ExpressionChip key="c" kind="node" node="Webhook" label="body.id" />, <ExpressionChip key="d" kind="node" node="Webhook" label="body.id" onRemove={() => undefined} />] },
        { label: 'raw', cells: [<ExpressionChip key="e" kind="raw" label="$json.total * 2" />, <ExpressionChip key="f" kind="raw" label="$json.total * 2" onRemove={() => undefined} />] },
      ]}
    />
  ),
};
