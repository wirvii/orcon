// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { Kbd } from '../kbd/Kbd';

import { CommandPalette, type PaletteCommand } from './CommandPalette';

const onSelect = fn();
const commands: PaletteCommand[] = [
  { id: 'new-flow', label: 'Create a flow', group: 'Flows', shortcut: ['N'], onSelect },
  { id: 'open-flow', label: 'Open a flow', group: 'Flows', onSelect },
  { id: 'executions', label: 'Go to executions', group: 'Navigate', keywords: ['runs', 'history'], onSelect },
  { id: 'inbox', label: 'Go to the attention inbox', group: 'Navigate', onSelect },
  { id: 'theme', label: 'Change theme', onSelect },
];

const meta = {
  title: 'Components/CommandPalette',
  component: CommandPalette,
  args: { commands },
  render: (args) => (
    <>
      <p className="text-fg-muted">
        Press <Kbd>Ctrl</Kbd> + <Kbd>K</Kbd> to open the palette.
      </p>
      <CommandPalette {...args} />
    </>
  ),
} satisfies Meta<typeof CommandPalette>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Closed. Ctrl+K opens it; typing filters; Enter runs the first match; Escape closes. */
export const Default: Story = {
  play: async () => {
    await userEvent.keyboard('{Control>}k{/Control}');
    const search = await within(document.body).findByRole('combobox', { name: 'Type a command or search' });
    await userEvent.type(search, 'history');
    await expect(await within(document.body).findByRole('option', { name: 'Go to executions' })).toBeInTheDocument();
    await userEvent.keyboard('{Enter}');
    await expect(onSelect).toHaveBeenCalled();
    await expect(within(document.body).queryByRole('dialog')).toBeNull();
  },
};
export const Open: Story = { args: { defaultOpen: true } };
export const Filtered: Story = { args: { defaultOpen: true, defaultSearch: 'flow' } };
export const Empty: Story = { args: { defaultOpen: true, defaultSearch: 'zzzz' } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="CommandPalette (open states are their own stories)"
      columns={['closed']}
      rows={[{ label: 'palette', cells: [<CommandPalette key="p" commands={commands} />] }]}
    />
  ),
};
