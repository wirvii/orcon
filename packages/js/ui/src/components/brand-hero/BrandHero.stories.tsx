// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { BrandHero } from './BrandHero';

const meta = {
  title: 'Components/BrandHero',
  component: BrandHero,
  parameters: {
    docs: { description: { component: 'The sign-in brand block: the approved lockup with the descriptor and the tagline as text.' } },
  },
} satisfies Meta<typeof BrandHero>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Matrix: Story = {
  render: () => <MatrixTable caption="BrandHero" columns={['default']} rows={[{ label: 'brand hero', cells: [<BrandHero key="h" />] }]} />,
};
