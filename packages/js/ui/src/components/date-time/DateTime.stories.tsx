// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { DateTime } from './DateTime';

// A fixed instant, so stories and baselines never change with the clock.
const instant = '2026-03-14T15:09:26.000Z';

const meta = {
  title: 'Components/DateTime',
  component: DateTime,
  args: { value: instant, timeStyle: 'short' },
} satisfies Meta<typeof DateTime>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // The ISO timestamp is available to machines and on hover.
    const time = canvasElement.querySelector('time');
    await expect(time).toHaveAttribute('datetime', instant);
    await expect(time).toHaveAttribute('title', instant);
    await expect(within(canvasElement).getByText(/2026/)).toBeInTheDocument();
  },
};
export const DateOnly: Story = { args: { timeStyle: undefined } };
export const TimeOnly: Story = { args: { timeOnly: true } };
export const TimeZone: Story = { args: { showTimeZone: true } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="DateTime (browser time zone)"
      columns={['default', 'date-only', 'time-zone']}
      rows={[
        {
          label: 'date time',
          cells: [
            <DateTime key="d" value={instant} timeStyle="short" />,
            <DateTime key="o" value={instant} />,
            <DateTime key="z" value={instant} timeStyle="short" showTimeZone />,
          ],
        },
      ]}
    />
  ),
};
