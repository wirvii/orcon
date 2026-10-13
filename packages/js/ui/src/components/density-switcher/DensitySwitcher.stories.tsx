// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { expect, userEvent, within } from 'storybook/test';

import type { Density } from '../../providers/DensityProvider';
import { MatrixTable } from '../../stories/MatrixTable';

import { DensitySwitcher } from './DensitySwitcher';
import { states } from './DensitySwitcher.meta';

const meta = {
  title: 'Components/DensitySwitcher',
  component: DensitySwitcher,
  render: (args) => {
    const [value, setValue] = useState<Density>('comfortable');
    return <DensitySwitcher {...args} value={value} onValueChange={setValue} />;
  },
} satisfies Meta<typeof DensitySwitcher>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole('radio', { name: 'Comfortable' })).toBeChecked();
    await userEvent.click(canvas.getByRole('radio', { name: 'Compact' }));
    await expect(canvas.getByRole('radio', { name: 'Compact' })).toBeChecked();
  },
};
export const Compact: Story = { args: { value: 'compact' }, render: (args) => <DensitySwitcher {...args} onValueChange={() => undefined} /> };

const values: Record<(typeof states)[number], Density> = { default: 'comfortable', compact: 'compact' };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="DensitySwitcher selections"
      columns={[...states]}
      rows={[{ label: 'selected', cells: states.map((state) => <DensitySwitcher key={state} value={values[state]} onValueChange={() => undefined} />) }]}
    />
  ),
};
