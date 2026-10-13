// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import type { ComponentProps, ComponentType } from 'react';
import { useIntl } from 'react-intl';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { storyMessages as m } from '../../stories/messages';
import { Button } from '../button/Button';
import { FormField } from '../form-field/FormField';
import { Input } from '../input/Input';

import { Dialog, DialogClose } from './Dialog';

type Args = Partial<ComponentProps<typeof Dialog>> & { withFooter?: boolean; withTrigger?: boolean };

/** The dialog with its sample text from the message catalog, so the pseudo-locale lengthens it. */
function RenameDialog({ withFooter, withTrigger, ...props }: Args) {
  const intl = useIntl();
  return (
    <Dialog
      title={intl.formatMessage(m.renameTitle)}
      trigger={withTrigger ? <Button variant="secondary">{intl.formatMessage(m.renameTitle)}</Button> : undefined}
      description={intl.formatMessage(m.renameDescription)}
      footer={
        withFooter ? (
          <>
            <DialogClose asChild>
              <Button variant="secondary">{intl.formatMessage(m.cancel)}</Button>
            </DialogClose>
            <Button>{intl.formatMessage(m.save)}</Button>
          </>
        ) : undefined
      }
      {...props}
    >
      <FormField label={intl.formatMessage(m.renameField)}>
        <Input defaultValue={intl.formatMessage(m.renameValue)} />
      </FormField>
    </Dialog>
  );
}

const meta: Meta<Args> = {
  title: 'Components/Dialog',
  component: Dialog as ComponentType<Args>,
  render: (args) => <RenameDialog {...args} />,
};

export default meta;
type Story = StoryObj<Args>;

/** Closed, with its trigger. Opening it moves focus inside; Escape returns focus to the trigger. */
export const Default: Story = {
  args: { withTrigger: true },
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
export const Footer: Story = { args: { defaultOpen: true, withFooter: true } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Dialog (the open states are their own stories)"
      columns={['default']}
      rows={[{ label: 'trigger', cells: [<RenameDialog key="d" withTrigger />] }]}
    />
  ),
};
