// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { Duration as DurationText, NumberFormat } from './NumberFormat';

const meta = {
  title: 'Components/NumberFormat',
  component: NumberFormat,
  args: { value: 1234567.891, maximumFractionDigits: 2 },
} satisfies Meta<typeof NumberFormat>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('1,234,567.89')).toBeInTheDocument();
  },
};
export const Percent: Story = { args: { value: 0.256, format: 'percent' } };
export const Currency: Story = { args: { value: 1234.5, format: 'currency', currency: 'USD' } };
export const Compact: Story = { args: { value: 1234567, compact: true } };
export const Duration: Story = { render: () => <DurationText ms={7_500_000} /> };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="NumberFormat and Duration"
      columns={['default', 'percent', 'currency', 'compact', 'duration']}
      rows={[
        {
          label: 'format',
          cells: [
            <NumberFormat key="d" value={1234567.891} maximumFractionDigits={2} />,
            <NumberFormat key="p" value={0.256} format="percent" />,
            <NumberFormat key="c" value={1234.5} format="currency" currency="USD" />,
            <NumberFormat key="k" value={1234567} compact />,
            <DurationText key="u" ms={7_500_000} />,
          ],
        },
      ]}
    />
  ),
};
