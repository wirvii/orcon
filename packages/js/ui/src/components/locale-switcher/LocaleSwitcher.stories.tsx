// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { expect, userEvent, within } from 'storybook/test';

import type { ProductLocale } from '../../i18n/locales';
import { MatrixTable } from '../../stories/MatrixTable';

import { LocaleSwitcher } from './LocaleSwitcher';

const meta = {
  title: 'Components/LocaleSwitcher',
  component: LocaleSwitcher,
  decorators: [(Story, context) => (context.name === 'Matrix' ? <Story /> : <div className="w-node-width"><Story /></div>)],
  render: (args) => {
    const [value, setValue] = useState<ProductLocale>('en');
    return <LocaleSwitcher {...args} value={value} onValueChange={setValue} />;
  },
} satisfies Meta<typeof LocaleSwitcher>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const trigger = within(canvasElement).getByRole('combobox', { name: 'Language' });
    await expect(trigger).toHaveTextContent('English');
    await userEvent.click(trigger);
    // Languages are listed by their own names.
    await userEvent.click(await within(document.body).findByRole('option', { name: 'Español' }));
    await expect(trigger).toHaveTextContent('Español');
  },
};
export const Spanish: Story = { args: { value: 'es' }, render: (args) => <LocaleSwitcher {...args} onValueChange={() => undefined} /> };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="LocaleSwitcher selections"
      columns={['default', 'spanish']}
      rows={[
        {
          label: 'selected',
          cells: [
            <div key="e" className="w-node-width">
              <LocaleSwitcher value="en" onValueChange={() => undefined} />
            </div>,
            <div key="s" className="w-node-width">
              <LocaleSwitcher value="es" onValueChange={() => undefined} />
            </div>,
          ],
        },
      ]}
    />
  ),
};
