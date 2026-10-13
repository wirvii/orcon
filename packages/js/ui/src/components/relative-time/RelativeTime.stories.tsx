// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { RelativeTime } from './RelativeTime';

const now = '2026-03-14T15:00:00.000Z';

const meta = {
  title: 'Components/RelativeTime',
  component: RelativeTime,
  args: { now, value: '2026-03-14T14:55:00.000Z' },
} satisfies Meta<typeof RelativeTime>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('5 minutes ago')).toBeInTheDocument();
  },
};
export const Future: Story = { args: { value: '2026-03-14T17:00:00.000Z' } };
export const Days: Story = { args: { value: '2026-03-09T15:00:00.000Z' } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="RelativeTime (now is fixed)"
      columns={['default', 'future', 'days']}
      rows={[
        {
          label: 'relative time',
          cells: [
            <RelativeTime key="d" now={now} value="2026-03-14T14:55:00.000Z" />,
            <RelativeTime key="f" now={now} value="2026-03-14T17:00:00.000Z" />,
            <RelativeTime key="x" now={now} value="2026-03-09T15:00:00.000Z" />,
          ],
        },
      ]}
    />
  ),
};
