// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Ellipsis } from 'lucide-react';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { IconButton } from '../icon-button/IconButton';

import { DropdownMenu, type MenuEntry } from './DropdownMenu';

const onSelect = fn();
const items: MenuEntry[] = [
  { type: 'label', label: 'Flow' },
  { type: 'item', label: 'Rename', onSelect },
  { type: 'item', label: 'Duplicate', onSelect },
  { type: 'item', label: 'Export', disabled: true },
  { type: 'separator' },
  { type: 'item', label: 'Delete flow', destructive: true, onSelect },
];

const trigger = <IconButton variant="secondary" icon={Ellipsis} label="Flow actions" />;

const meta = {
  title: 'Components/DropdownMenu',
  component: DropdownMenu,
  args: { trigger, items },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
        <div className="h-node-width">
          <Story />
        </div>
      ),
  ],
} satisfies Meta<typeof DropdownMenu>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // Keyboard path: focus the button, open with Enter, move with the arrow keys, select with Enter.
    await userEvent.tab();
    await userEvent.keyboard('{Enter}');
    await within(document.body).findByRole('menu');
    await userEvent.keyboard('{ArrowDown}{Enter}');
    await expect(onSelect).toHaveBeenCalled();
    await expect(within(canvasElement).getByRole('button', { name: 'Flow actions' })).toHaveFocus();
  },
};

export const Open: Story = { args: { defaultOpen: true } };
export const Destructive: Story = { args: { defaultOpen: true, items: items.filter((entry) => entry.type !== 'item' || entry.destructive) } };
export const Disabled: Story = { args: { defaultOpen: true, items: items.map((entry) => (entry.type === 'item' ? { ...entry, disabled: true } : entry)) } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="DropdownMenu states"
      columns={['default', 'open']}
      rows={[
        {
          label: 'menu',
          cells: [
            <div key="d" className="h-12">
              <DropdownMenu trigger={<IconButton variant="secondary" icon={Ellipsis} label="Actions, closed" />} items={items} />
            </div>,
            <div key="o" className="h-node-width w-node-width">
              <DropdownMenu defaultOpen trigger={<IconButton variant="secondary" icon={Ellipsis} label="Actions, open" />} items={items} />
            </div>,
          ],
        },
      ]}
    />
  ),
};
