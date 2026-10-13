// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { Spinner } from './Spinner';

const meta = {
  title: 'Components/Spinner',
  component: Spinner,
  argTypes: { size: { control: 'inline-radio', options: [16, 20, 24] } },
} satisfies Meta<typeof Spinner>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Spinner sizes"
      columns={['labeled', 'decorative']}
      rows={([16, 20, 24] as const).map((size) => ({
        label: `${size} px`,
        cells: [<Spinner key="labeled" size={size} />, <Spinner key="decorative" size={size} decorative />],
      }))}
    />
  ),
};
