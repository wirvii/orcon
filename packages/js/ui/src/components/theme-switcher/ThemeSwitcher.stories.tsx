// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { expect, userEvent, within } from 'storybook/test';

import type { ThemePreference } from '../../providers/ThemeProvider';
import { MatrixTable } from '../../stories/MatrixTable';

import { ThemeSwitcher } from './ThemeSwitcher';
import { states } from './ThemeSwitcher.meta';

const meta = {
  title: 'Components/ThemeSwitcher',
  component: ThemeSwitcher,
  // Stories keep their own value so the toolbar theme stays in charge of the page.
  render: (args) => {
    const [value, setValue] = useState<ThemePreference>('system');
    return <ThemeSwitcher {...args} value={value} onValueChange={setValue} />;
  },
} satisfies Meta<typeof ThemeSwitcher>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole('radio', { name: 'System' })).toBeChecked();
    await userEvent.click(canvas.getByRole('radio', { name: 'Dark' }));
    await expect(canvas.getByRole('radio', { name: 'Dark' })).toBeChecked();
  },
};
export const Light: Story = { args: { value: 'light' }, render: (args) => <ThemeSwitcher {...args} onValueChange={() => undefined} /> };
export const Dark: Story = { args: { value: 'dark' }, render: (args) => <ThemeSwitcher {...args} onValueChange={() => undefined} /> };

const values: Record<(typeof states)[number], ThemePreference> = { default: 'system', light: 'light', dark: 'dark' };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="ThemeSwitcher selections"
      columns={[...states]}
      rows={[
        {
          label: 'selected',
          cells: states.map((state) => <ThemeSwitcher key={state} value={values[state]} onValueChange={() => undefined} />),
        },
      ]}
    />
  ),
};
