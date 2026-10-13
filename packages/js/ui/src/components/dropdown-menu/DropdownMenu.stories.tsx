// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Ellipsis } from 'lucide-react';
import type { ComponentProps, ComponentType } from 'react';
import { useIntl, type MessageDescriptor } from 'react-intl';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { storyMessages as m } from '../../stories/messages';
import { IconButton } from '../icon-button/IconButton';

import { DropdownMenu, type MenuEntry } from './DropdownMenu';

const onSelect = fn();

type Args = Partial<ComponentProps<typeof DropdownMenu>> & { only?: 'destructive' | 'disabled'; triggerLabel?: MessageDescriptor };

/** The menu with its sample text from the message catalog, so the pseudo-locale lengthens it. */
function FlowMenu({ only, triggerLabel = m.menuTrigger, ...props }: Args) {
  const intl = useIntl();
  const items: MenuEntry[] = [
    { type: 'label', label: intl.formatMessage(m.menuLabel) },
    { type: 'item', label: intl.formatMessage(m.menuRename), onSelect },
    { type: 'item', label: intl.formatMessage(m.menuDuplicate), onSelect },
    { type: 'item', label: intl.formatMessage(m.menuExport), disabled: true },
    { type: 'separator' },
    { type: 'item', label: intl.formatMessage(m.deleteFlow), destructive: true, onSelect },
  ];
  const shown =
    only === 'destructive'
      ? items.filter((entry) => entry.type !== 'item' || entry.destructive)
      : only === 'disabled'
        ? items.map((entry) => (entry.type === 'item' ? { ...entry, disabled: true } : entry))
        : items;
  return <DropdownMenu trigger={<IconButton variant="secondary" icon={Ellipsis} label={intl.formatMessage(triggerLabel)} />} items={shown} {...props} />;
}

const meta: Meta<Args> = {
  title: 'Components/DropdownMenu',
  component: DropdownMenu as ComponentType<Args>,
  render: (args) => <FlowMenu {...args} />,
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
        <div className="h-panel-width">
          <Story />
        </div>
      ),
  ],
};

export default meta;
type Story = StoryObj<Args>;

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
export const Destructive: Story = { args: { defaultOpen: true, only: 'destructive' } };
export const Disabled: Story = { args: { defaultOpen: true, only: 'disabled' } };

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
              <FlowMenu triggerLabel={m.menuClosed} />
            </div>,
            <div key="o" className="h-panel-width w-panel-width">
              <FlowMenu defaultOpen triggerLabel={m.menuOpen} />
            </div>,
          ],
        },
      ]}
    />
  ),
};
