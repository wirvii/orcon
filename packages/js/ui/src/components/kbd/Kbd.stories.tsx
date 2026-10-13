// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { Kbd, KbdCombination } from './Kbd';

const meta = {
  title: 'Components/Kbd',
  component: Kbd,
  args: { children: 'K' },
} satisfies Meta<typeof Kbd>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Combination: Story = {
  render: () => <KbdCombination keys={['Ctrl', 'Shift', 'K']} />,
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Kbd"
      columns={['default', 'combination']}
      rows={[{ label: 'kbd', cells: [<Kbd key="k">Enter</Kbd>, <KbdCombination key="c" keys={['Ctrl', 'K']} />] }]}
    />
  ),
};
