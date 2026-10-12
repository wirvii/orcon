// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import type { ComponentProps, ComponentType } from 'react';
import { useIntl } from 'react-intl';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { storyMessages as m } from '../../stories/messages';
import { Kbd } from '../kbd/Kbd';

import { CommandPalette, type PaletteCommand } from './CommandPalette';

const onSelect = fn();

type Args = Partial<ComponentProps<typeof CommandPalette>>;

/** The palette with its sample commands from the message catalog, so the pseudo-locale lengthens them. */
function SamplePalette(props: Args) {
  const intl = useIntl();
  const flows = intl.formatMessage(m.paletteGroupFlows);
  const navigate = intl.formatMessage(m.paletteGroupNavigate);
  const commands: PaletteCommand[] = [
    { id: 'new-flow', label: intl.formatMessage(m.paletteCreate), group: flows, shortcut: ['N'], onSelect },
    { id: 'open-flow', label: intl.formatMessage(m.paletteOpenFlow), group: flows, onSelect },
    { id: 'executions', label: intl.formatMessage(m.paletteExecutions), group: navigate, keywords: ['runs', 'history'], onSelect },
    { id: 'inbox', label: intl.formatMessage(m.paletteInbox), group: navigate, onSelect },
    { id: 'theme', label: intl.formatMessage(m.paletteTheme), onSelect },
  ];
  return <CommandPalette commands={commands} {...props} />;
}

function Hint() {
  const intl = useIntl();
  return (
    <p className="text-fg-muted">
      {intl.formatMessage(m.paletteHint, { first: <Kbd key="a">Ctrl</Kbd>, second: <Kbd key="b">K</Kbd> })}
    </p>
  );
}

const meta: Meta<Args> = {
  title: 'Components/CommandPalette',
  component: CommandPalette as ComponentType<Args>,
  render: (args) => (
    <>
      <Hint />
      <SamplePalette {...args} />
    </>
  ),
};

export default meta;
type Story = StoryObj<Args>;

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
    <MatrixTable caption="CommandPalette (open states are their own stories)" columns={['closed']} rows={[{ label: 'palette', cells: [<SamplePalette key="p" />] }]} />
  ),
};
