// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { BrandMark } from './BrandMark';

const meta = {
  title: 'Components/BrandMark',
  component: BrandMark,
  parameters: {
    docs: {
      description: {
        component:
          'The approved lockup and symbol. The file that fits the active theme is chosen by CSS: light type on dark, the dark rounded square with navy type on light.',
      },
    },
  },
} satisfies Meta<typeof BrandMark>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Symbol: Story = { args: { variant: 'symbol' } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="BrandMark variants"
      columns={['default', 'symbol']}
      rows={[{ label: 'brand mark', cells: [<BrandMark key="l" />, <BrandMark key="s" variant="symbol" />] }]}
    />
  ),
};
