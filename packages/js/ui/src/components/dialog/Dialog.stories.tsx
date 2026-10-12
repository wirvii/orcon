// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { Button } from '../button/Button';
import { FormField } from '../form-field/FormField';
import { Input } from '../input/Input';

import { Dialog, DialogClose } from './Dialog';

const footer = (
  <>
    <DialogClose asChild>
      <Button variant="secondary">Cancel</Button>
    </DialogClose>
    <Button>Save</Button>
  </>
);

const meta = {
  title: 'Components/Dialog',
  component: Dialog,
  args: {
    title: 'Rename flow',
    description: 'The new name appears in the flow list and in notifications.',
    children: (
      <FormField label="Flow name">
        <Input defaultValue="Nightly sync" />
      </FormField>
    ),
  },
} satisfies Meta<typeof Dialog>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Closed, with its trigger. Opening it moves focus inside; Escape returns focus to the trigger. */
export const Default: Story = {
  args: { trigger: <Button variant="secondary">Rename flow</Button> },
  play: async ({ canvasElement }) => {
    const trigger = within(canvasElement).getByRole('button', { name: 'Rename flow' });
    await userEvent.click(trigger);
    const dialog = await within(document.body).findByRole('dialog', { name: 'Rename flow' });
    await expect(dialog).toHaveAccessibleDescription(/The new name appears/);
    await userEvent.keyboard('{Escape}');
    await expect(trigger).toHaveFocus();
  },
};

export const Open: Story = { args: { defaultOpen: true } };
export const Footer: Story = { args: { defaultOpen: true, footer } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Dialog (the open states are their own stories)"
      columns={['default']}
      rows={[
        {
          label: 'trigger',
          cells: [<Dialog key="d" title="Rename flow" trigger={<Button variant="secondary">Rename flow</Button>} />],
        },
      ]}
    />
  ),
};
