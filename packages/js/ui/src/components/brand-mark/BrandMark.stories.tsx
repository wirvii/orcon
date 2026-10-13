// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { BrandMark } from './BrandMark';

const meta = {
  title: 'Components/BrandMark',
  component: BrandMark,
  parameters: {
    docs: { description: { component: 'Placeholder: the approved symbol and wordmark are delivered separately.' } },
  },
} satisfies Meta<typeof BrandMark>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Symbol: Story = { args: { variant: 'symbol' } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="BrandMark variants (neutral placeholder)"
      columns={['default', 'symbol']}
      rows={[{ label: 'brand mark', cells: [<BrandMark key="l" />, <BrandMark key="s" variant="symbol" />] }]}
    />
  ),
};
